package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig()

	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создаём клиента MEXC
	ex := exchange.NewMEXCExchange(cfg.APIKey, cfg.SecretKey, cfg.Symbol)

	// Создаём контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал для получения цен
	priceChan := make(chan float64, 100)

	// Запускаем подписку на цены
	log.Println("Запуск подписки на цены...")
	go ex.SubscribePrice(ctx, priceChan)

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Основной цикл вывода
	go func() {
		for {
			select {
			case price := <-priceChan:
				log.Printf("Price Update: %.8f", price)
			case <-ctx.Done():
				log.Println("Остановка подписки на цены...")
				return
			}
		}
	}()

	// Ждём сигнала завершения
	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем...")
	cancel()
}
