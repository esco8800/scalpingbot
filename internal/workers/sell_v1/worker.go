package sell_v1

import (
	"context"
	"log"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"strconv"
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
		if b.storage.Has(order.OrderID) && order.Status == exchange.Filled {
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
			log.Printf("Ордер на продажу размещен: %s", orderResp.OrderID)
			b.storage.Remove(order.OrderID)
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
