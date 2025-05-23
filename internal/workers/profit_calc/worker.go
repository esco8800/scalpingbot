package profit_calc

import (
	"context"
	"fmt"
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
	storage  repo.ProfitRepo
}

// NewBot - конструктор бота
func NewBot(cfg config.Config, ex exchange.Exchange, storage repo.ProfitRepo) *Bot {
	return &Bot{
		config:   cfg,
		exchange: ex,
		storage:  storage,
	}
}

func (b *Bot) Process(ctx context.Context) error {
	var allOrders []exchange.OrderInfo
	now := time.Now()
	endTime := now.UnixMilli()
	startTime := now.Add(-59 * 24 * 7 * time.Minute).UnixMilli()

	for {
		orders, err := b.exchange.GetAllOrders(ctx, b.config.Symbol, startTime, endTime)
		if err != nil {
			return fmt.Errorf("ошибка при получении ордеров: %w", err)
		}

		if len(orders) == 0 {
			break
		}

		allOrders = append(allOrders, orders...)

		// Найти минимальный Time среди ордеров
		minTime := orders[0].Time
		for _, o := range orders {
			if o.Time < minTime {
				minTime = o.Time
			}
		}

		// Сдвигаем endTime назад
		endTime = minTime - 1

		// Проверка выхода за пределы допустимого диапазона
		if endTime < startTime {
			break
		}

		time.Sleep(200 * time.Millisecond) // для обхода rate-limit
	}

	b.storage.Add(repo.ProfitKey, tools.CalculateSellVolumeInUSDT(allOrders)*b.config.ProfitPercent)

	return nil
}

func (b *Bot) Name() string {
	return "profit_calc"
}
