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
	buyCount, sellCount := getCountOrders(allOrders)
	log.Printf("Открытых ордеров на покупку: %d", buyCount)
	log.Printf("Открытых ордеров на продажу: %d", sellCount)

	for _, order := range allOrders {
		orderAge := time.Now().Sub(time.UnixMilli(order.Time))
		// Если ордер уже закрыт, то открываем продажу
		if b.storage.Has(order.OrderID) && order.Status == exchange.Filled && orderAge > 15*time.Second {
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
			orderResp, err := b.exchange.PlaceOrder(ctx, sellOrder)
			if err != nil {
				return err
			}
			log.Printf("Ордер на продажу из воркера размещен: %s", orderResp.OrderID)
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
				// сначала отменяем старый ордер
				err := b.exchange.CancelOrder(ctx, b.config.Symbol, order.OrderID)
				if err != nil {
					log.Printf("Ошибка отмены старого ордера %s: %v", order.OrderID, err)
					return err
				}
				// затем создаем новый ордер на продажу на сумму заполненной части по текущей цене (тк она по идее выше цены старого ордера)
				newPrice, err := b.exchange.GetPrice(ctx, "KASUSDT")
				if err != nil {
					return err
				}
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

func getCountOrders(allOrders []exchange.OrderInfo) (int, int) {
	buyCount := 0
	sellCount := 0

	for _, order := range allOrders {
		if order.Status == exchange.New || order.Status == exchange.PartiallyFilled {
			switch order.Side {
			case exchange.Buy:
				buyCount++
			case exchange.Sell:
				sellCount++
			}
		}
	}
	return buyCount, sellCount
}

func (b *Bot) Name() string {
	return "sell_v1"
}
