package listener

import (
	"context"
	"fmt"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/logger"
	"sync"
)

// OrderListener - компонент для обработки обновлений ордеров
type OrderListener struct {
	updateCh <-chan exchange.OrderUpdate
	logger   logger.Logger
	wg       sync.WaitGroup
}

// NewOrderListener - конструктор листенера
func NewOrderListener(updateCh <-chan exchange.OrderUpdate, logLogger logger.Logger) *OrderListener {
	return &OrderListener{
		updateCh: updateCh,
		logger:   logLogger,
	}
}

// Start - запуск обработки сообщений
func (l *OrderListener) Start(ctx context.Context) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		for {
			select {
			case <-ctx.Done():
				l.logger.Info(fmt.Sprintf("Shutting down order listener: %v", ctx.Err()))
				return
			case update, ok := <-l.updateCh:
				if !ok {
					l.logger.Info("Update channel closed, shutting down")
					return
				}
				l.processUpdate(update)
			}
		}
	}()
}

// processUpdate - обработка одного обновления
func (l *OrderListener) processUpdate(update exchange.OrderUpdate) {
	// Логирование в зависимости от статуса
	switch update.Status {
	case "NEW":
		l.logger.Info(fmt.Sprintf("New order: Symbol=%s, OrderId=%s, Price=%s, Quantity=%s",
			update.Symbol, update.OrderId, update.Price, update.Quantity))
	case "FILLED":
		l.logger.Info(fmt.Sprintf("Order filled: Symbol=%s, OrderId=%s, Price=%s, Quantity=%s",
			update.Symbol, update.OrderId, update.Price, update.Quantity))
	case "CANCELED":
		l.logger.Info(fmt.Sprintf("Order canceled: Symbol=%s, OrderId=%s", update.Symbol, update.OrderId))
	case "PARTIALLY_FILLED":
		l.logger.Info(fmt.Sprintf("Order partially filled: Symbol=%s, OrderId=%s, Price=%s, Quantity=%s",
			update.Symbol, update.OrderId, update.Price, update.Quantity))
	default:
		l.logger.Info(fmt.Sprintf("Unknown order status: Symbol=%s, OrderId=%s, Status=%s",
			update.Symbol, update.OrderId, update.Status))
	}
}

// Wait - ожидание завершения работы листенера
func (l *OrderListener) Wait() {
	l.wg.Wait()
}
