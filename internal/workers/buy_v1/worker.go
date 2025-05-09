package buy_v1

import (
	"context"
	"log"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"scalpingbot/internal/tools"
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
	// чекаем тренд и ждем
	err := b.SleepTimeout(ctx)
	if err != nil {
		return err
	}

	accountInfo, err := b.exchange.GetAccountInfo(ctx)
	if err != nil {
		return err
	}
	usdtBalance, err := accountInfo.GetUsdtBalance()
	if err != nil {
		return err
	}
	price, err := b.exchange.GetPrice(ctx, "KASUSDT")
	if err != nil {
		return err
	}
	log.Printf("Текущая цена KAS/USDT: %.6f\n", price)
	log.Printf("Баланс usdt: %v", usdtBalance)

	if usdtBalance > (b.config.OrderSize * price) {
		order := exchange.SpotOrderRequest{
			Symbol:   b.config.Symbol,
			Side:     exchange.Buy,
			Type:     exchange.Limit,
			Quantity: b.config.OrderSize,
			Price:    price,
		}
		orderResp, err := b.exchange.PlaceOrder(ctx, order)
		if err != nil {
			return err
		}
		b.storage.Add(orderResp.OrderID)
		log.Printf("Ордер на покупку размещен: %s", orderResp.OrderID)
	} else {
		log.Printf("Баланс usdt меньше заданного размера ордера, ожидание...")
		time.Sleep(time.Second * 15)
	}
	return nil
}

func (b *Bot) SleepTimeout(ctx context.Context) error {
	// получаем klines
	klines, err := b.exchange.GetKlines(ctx, b.config.Symbol, exchange.KlineInterval1m, 10)
	if err != nil {
		return err
	}

	timeout := tools.AdjustTimeout(b.config.BaseBuyTimeout, klines)
	log.Printf("Спим таймаут: %d", timeout)
	time.Sleep(time.Second * time.Duration(timeout))
	return nil
}

func (b *Bot) Name() string {
	return "buy_v1"
}
