package listener

import (
	"context"
	"fmt"
	"log"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/logger"
	"scalpingbot/internal/repo"
	"strconv"
	"sync"
)

// OrderListener - компонент для обработки обновлений ордеров
type OrderListener struct {
	cfg      config.Config
	exchange exchange.Exchange
	updateCh <-chan exchange.OrderUpdate
	logger   logger.Logger
	storage  repo.Repo
	wg       sync.WaitGroup
}

// NewOrderListener - конструктор листенера
func NewOrderListener(cfg config.Config, ex exchange.Exchange, updateCh <-chan exchange.OrderUpdate, logLogger logger.Logger, storage repo.Repo) *OrderListener {
	return &OrderListener{
		cfg:      cfg,
		exchange: ex,
		updateCh: updateCh,
		logger:   logLogger,
		storage:  storage,
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
				l.processUpdate(ctx, update)
			}
		}
	}()
}

// processUpdate - обработка одного обновления
func (l *OrderListener) processUpdate(ctx context.Context, update exchange.OrderUpdate) {
	if l.storage.Has(update.OrderId) && update.Status == exchange.FullyTraded {
		// Логирование ордера
		l.logger.Info(fmt.Sprintf("New order full update: OrderId=%s, Price=%s, Quantity=%s Status=%d",
			update.OrderId, update.Price, update.Quantity, update.Status))

		newPrice, err := strconv.ParseFloat(update.Price, 64)
		if err != nil {
			log.Printf("Error parsing price: %v", err)
			l.logger.Error(fmt.Sprintf("Error parsing price: %v", err))
			return
		}
		newPrice = newPrice * (1 + l.cfg.ProfitPercent/100)
		qty, err := strconv.ParseFloat(update.Quantity, 64)
		if err != nil {
			log.Printf("Error parsing quantity: %v", err)
			l.logger.Error(fmt.Sprintf("Error parsing quantity: %v", err))
			return
		}
		sellOrder := exchange.SpotOrderRequest{
			Symbol:   l.cfg.Symbol,
			Side:     exchange.Sell,
			Type:     exchange.Limit,
			Quantity: qty,
			Price:    newPrice,
		}
		orderResp, err := l.exchange.PlaceOrder(ctx, sellOrder)
		if err != nil {
			log.Printf("Error placing sell order: %v", err)
			l.logger.Error(fmt.Sprintf("Error placing sell order: %v", err))
			return
		}
		log.Printf("Ордер в лиснере на продажу размещен: %s oldPrice=%s newPrice=%s", orderResp.OrderID, update.Price, orderResp.Price)
		// Удаляем старый бай ордер из стораджа
		l.storage.Remove(update.OrderId)
	}
}

// Wait - ожидание завершения работы листенера
func (l *OrderListener) Wait() {
	l.wg.Wait()
}
