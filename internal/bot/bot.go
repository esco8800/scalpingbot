package bot

import (
	"context"
	"log"
	"time"

	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
)

// Bot - структура бота
type Bot struct {
	config    config.Config
	exchange  exchange.Exchange
	orders    []Order
	priceChan chan float64
	orderChan chan exchange.OrderUpdate
}

// NewBot - конструктор бота
func NewBot(cfg config.Config, ex exchange.Exchange) *Bot {
	return &Bot{
		config:    cfg,
		exchange:  ex,
		orders:    []Order{},
		priceChan: make(chan float64, 100),
		orderChan: make(chan exchange.OrderUpdate, 100),
	}
}

// Start - запуск бота с поддержкой контекста
func (b *Bot) Start(ctx context.Context) {
	log.Println("Бот запущен командой 'scalpingbot'")

	// Подписываемся на WebSocket
	go b.exchange.SubscribePrice(ctx, b.priceChan)
	go b.exchange.SubscribeOrders(ctx, b.orderChan)

	// Ждём первую цену
	currentPrice := b.waitForFirstPrice(ctx)
	if currentPrice == 0 {
		log.Println("Не удалось получить начальную цену, завершение...")
		return
	}

	// Первая покупка
	b.placeBuyOrder(ctx, currentPrice)

	// Запускаем основной цикл
	b.run(ctx)
}

// waitForFirstPrice - ждёт первую цену из WebSocket
func (b *Bot) waitForFirstPrice(ctx context.Context) float64 {
	for {
		select {
		case price := <-b.priceChan:
			return price
		case <-ctx.Done():
			return 0
		case <-time.After(5 * time.Second):
			log.Println("Таймаут ожидания первой цены")
			return 0
		}
	}
}

// placeBuyOrder - выставление ордера на покупку
func (b *Bot) placeBuyOrder(ctx context.Context, price float64) {
	if b.config.AvailableDeposit < b.config.OrderSize {
		log.Println("Недостаточно средств для покупки!")
		return
	}

	orderID, err := b.exchange.PlaceBuyOrder(ctx, price, b.config.OrderSize)
	if err != nil {
		log.Printf("Ошибка покупки: %v", err)
		return
	}

	sellPrice := price * (1 + b.config.ProfitPercent/100)
	sellOrderID, err := b.exchange.PlaceSellOrder(ctx, sellPrice, b.config.OrderSize)
	if err != nil {
		log.Printf("Ошибка продажи: %v", err)
		return
	}

	order := Order{
		BuyPrice:  price,
		SellPrice: sellPrice,
		Amount:    b.config.OrderSize / price,
		Active:    true,
	}
	b.orders = append(b.orders, order)
	b.config.AvailableDeposit -= b.config.OrderSize

	log.Printf("Покупка: %.2f$ по цене %.2f, продажа за %.2f (ID: %s)", b.config.OrderSize, price, sellPrice, sellOrderID)
}

// run - основной цикл бота
func (b *Bot) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Остановка основного цикла бота...")
			return
		case currentPrice := <-b.priceChan:
			// Проверка падения цены
			lastBuyPrice := b.getLastBuyPrice()
			if lastBuyPrice > 0 && currentPrice <= lastBuyPrice*(1-b.config.DropPercent/100) {
				b.placeBuyOrder(ctx, currentPrice)
			}
		case orderUpdate := <-b.orderChan:
			// Обработка обновлений ордеров
			for i := range b.orders {
				if b.orders[i].Active && orderUpdate.Price == b.orders[i].SellPrice && orderUpdate.Status == "FILLED" {
					b.orders[i].Active = false
					profit := b.config.OrderSize * (b.config.ProfitPercent / 100)
					b.config.AvailableDeposit += b.config.OrderSize + profit
					log.Printf("Ордер исполнен! Продано за %.2f, прибыль: %.2f", orderUpdate.Price, profit)

					if sleepWithContext(ctx, time.Duration(b.config.DelaySeconds)*time.Second) {
						currentPrice, err := b.exchange.GetCurrentPrice(ctx)
						if err != nil {
							log.Printf("Ошибка получения цены: %v", err)
							continue
						}
						b.placeBuyOrder(ctx, currentPrice)
					}
				}
			}
		}
	}
}

// sleepWithContext - задержка с учётом отмены контекста
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		log.Println("Прерывание задержки из-за завершения...")
		return false
	case <-time.After(d):
		return true
	}
}

// getLastBuyPrice - получение последней цены покупки
func (b *Bot) getLastBuyPrice() float64 {
	if len(b.orders) == 0 {
		return 0
	}
	return b.orders[len(b.orders)-1].BuyPrice
}
