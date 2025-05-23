package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"scalpingbot/internal/logger"
	"syscall"

	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
)

func main() {
	// Создаём контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	logLoger := logger.SetupLogger(cfg.TgToken, cfg.TgChatID)
	// Создаём клиента MEXC
	ex := exchange.NewMEXCClient(cfg.APIKey, cfg.SecretKey, cfg.Symbol, logLoger)
	req := exchange.SpotOrderRequest{
		Side:     exchange.Sell,
		Type:     "LIMIT",
		Quantity: 10,
		Price:    1,
	}
	order, err := ex.PlaceOrder(ctx, req)
	if err != nil {
		log.Fatalf("Ошибка создания ордера: %v", err)
	}
	log.Printf("Ордер создан: %v", order)

	// Канал для получения обновлений ордеров
	//orderChan := make(chan exchange.OrderUpdate, 100)

	// Запускаем подписку на ордера
	//log.Println("Запуск подписки на ордера...")
	//go ex.SubscribeOrders(ctx, orderChan)

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Основной цикл вывода
	//go func() {
	//	for {
	//		select {
	//		case order := <-orderChan:
	//			log.Printf("Order Update: OrderID=%s, Price=%.8f, Quantity=%.8f, Status=%s, Timestamp=%d",
	//				order.OrderID, order.Price, order.Quantity, order.Status, order.Timestamp)
	//		case <-ctx.Done():
	//			log.Println("Остановка подписки на ордера...")
	//			return
	//		}
	//	}
	//}()

	// Ждём сигнала завершения
	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем...")
	cancel()
}
