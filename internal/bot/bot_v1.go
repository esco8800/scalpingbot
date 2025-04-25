package bot

import (
	"context"

	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
)

// WorkerFunc — тип для воркера
type WorkerFunc func(ctx context.Context) error

// Bot - структура бота
type Bot struct {
	config   config.Config
	exchange exchange.Exchange
}

// NewBot - конструктор бота
func NewBot(cfg config.Config, ex exchange.Exchange) *Bot {
	return &Bot{
		config:   cfg,
		exchange: ex,
	}
}

func (b *Bot) Process(ctx context.Context) error {
	return nil
}

func (b *Bot) Name(ctx context.Context) string {
	return "bot"
}
