package sell_v1

import (
	"context"
	"log"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"strconv"
	"time"
)

// WorkerFunc — тип для воркера
type WorkerFunc func(ctx context.Context) error

// Bot - структура бота
type Bot struct {
	config   config.Config
	exchange exchange.Exchange
	storage  repo.Repo
}

// NewBot - конструктор бота
func NewBot(cfg config.Config, ex exchange.Exchange, storage repo.Repo) *Bot {
	return &Bot{
		config:   cfg,
		exchange: ex,
		storage:  storage,
	}
}

func (b *Bot) Process(ctx context.Context) error {
	allOrders, err := b.exchange.GetAllOrders(ctx, b.config.Symbol)
	if err != nil {
		return err
	}
	openOrders, err := b.exchange.GetOpenOrders(ctx, b.config.Symbol)
	if err != nil {
		return err
	}
	buyCount, sellCount := GetCountOpenOrders(openOrders)
	log.Printf("Открытых ордеров на покупку: %d", buyCount)
	log.Printf("Открытых ордеров на продажу: %d", sellCount)

	// Проверяем, что не превышено количество открытых ордеров
	if buyCount+sellCount >= exchange.MaxOpenOrders {
		log.Printf("Превышено количество открытых ордеров: %d", buyCount+sellCount)
		return nil
	}

	accountInfo, err := b.exchange.GetAccountInfo(ctx)
	if err != nil {
		return err
	}
	kasFreeBalance, err := accountInfo.GetKasBalance()
	if err != nil {
		return err
	}

	for _, order := range allOrders {
		orderAge := time.Now().Sub(time.UnixMilli(order.Time))
		updateTime := time.Now().Sub(time.UnixMilli(order.UpdateTime))
		// Процесим ордера, которые незапроцессились лиснером
		if b.storage.Has(order.OrderID) && order.Status == exchange.Filled && updateTime > 15*time.Second {
			newPrice, err := strconv.ParseFloat(order.Price, 64)
			if err != nil {
				return err
			}
			newPrice = newPrice * (1 + b.config.ProfitPercent/100)
			qty, err := strconv.ParseFloat(order.ExecutedQty, 64)
			if err != nil {
				return err
			}
			sellOrder := exchange.SpotOrderRequest{
				Symbol:   b.config.Symbol,
				Side:     exchange.Sell,
				Type:     exchange.Limit,
				Quantity: qty,
				Price:    newPrice,
			}

			// Проверяем, что есть достаточно KAS
			if kasFreeBalance < qty {
				log.Printf("Недостаточно KAS для продажи, пропускаем ордер: %s (статус: %s, возраст: %s)", order.OrderID, order.Status, orderAge)
				continue
			}

			orderResp, err := b.exchange.PlaceOrder(ctx, sellOrder)
			if err != nil {
				log.Printf("Ошибка размещения ордера на продажу из воркера: %v", err)
				return err
			}
			log.Printf("Ордер на продажу из воркера размещен: %s OldPrice=%s NewPrice=%s", orderResp.OrderID, order.Price, orderResp.Price)
			// Удаляем ордер из стораджа
			b.storage.Remove(order.OrderID)
		}

		// Отмена старых незаполненных ордеров
		if b.storage.Has(order.OrderID) && (order.Status == exchange.New) {
			if orderAge > 10*time.Minute {
				err := b.exchange.CancelOrder(ctx, b.config.Symbol, order.OrderID)
				if err != nil {
					log.Printf("Ошибка отмены старого ордера %s: %v", order.OrderID, err)
					return err
				}
				// Удаляем ордер из стораджа
				b.storage.Remove(order.OrderID)
				log.Printf("Старый ордер отменён: %s (статус: %s, возраст: %s)", order.OrderID, order.Status, orderAge)
			}
		}

		// Отмена старых частично заполненных ордеров
		if b.storage.Has(order.OrderID) && order.Status == exchange.PartiallyFilled {
			if orderAge > 10*time.Minute {
				// Проверяем, что сумма больше минимальной
				qty, err := strconv.ParseFloat(order.ExecutedQty, 64)
				if err != nil {
					return err
				}
				newPrice, err := b.exchange.GetPrice(ctx, "KASUSDT")
				if err != nil {
					return err
				}

				// Проверяем, что сумма больше минимальной и есть достаточно KAS
				if qty*newPrice < 1 {
					log.Printf("Сумма меньше минимальной, ордер не отменён: %s (статус: %s, возраст: %s)", order.OrderID, order.Status, orderAge)
					continue
				}
				if kasFreeBalance < qty {
					log.Printf("Недостаточно KAS для продажи, пропускаем ордер: %s (статус: %s, возраст: %s)", order.OrderID, order.Status, orderAge)
					continue
				}

				// сначала отменяем старый ордер
				err = b.exchange.CancelOrder(ctx, b.config.Symbol, order.OrderID)
				if err != nil {
					log.Printf("Ошибка отмены старого ордера %s: %v", order.OrderID, err)
					return err
				}
				// затем создаем новый ордер на продажу на сумму заполненной части по текущей цене (тк она по идее выше цены старого ордера)
				sellOrder := exchange.SpotOrderRequest{
					Symbol:   b.config.Symbol,
					Side:     exchange.Sell,
					Type:     exchange.Limit,
					Quantity: qty,
					Price:    newPrice,
				}
				orderResp, err := b.exchange.PlaceOrder(ctx, sellOrder)
				if err != nil {
					return err
				}
				log.Printf("Ордер на продажу от частичного: %s oldPrice: %s newPrice %.6f", orderResp.OrderID, order.Price, newPrice)
				// Удаляем ордер из стораджа
				b.storage.Remove(order.OrderID)
				log.Printf("Старый ордер отменён: %s (статус: %s, возраст: %s)", order.OrderID, order.Status, orderAge)
			}
		}
	}
	return nil
}

func GetCountOpenOrders(orders []exchange.OrderInfo) (int, int) {
	buyCount := 0
	sellCount := 0

	// Считаем количество BUY и SELL ордеров
	for _, order := range orders {
		switch order.Side {
		case "BUY":
			buyCount++
		case "SELL":
			sellCount++
		}
	}

	return buyCount, sellCount
}

func (b *Bot) Name() string {
	return "sell_v1"
}
