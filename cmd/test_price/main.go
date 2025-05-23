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
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig()

	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	logLoger := logger.SetupLogger(cfg.TgToken, cfg.TgChatID)
	// Создаём клиента MEXC
	ex := exchange.NewMEXCClient(cfg.APIKey, cfg.SecretKey, cfg.Symbol, logLoger)

	// Создаём контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал для получения цен
	priceChan := make(chan float64, 100)

	// Запускаем подписку на цены
	//log.Println("Запуск подписки на цены...")
	//err = ex.SubscribePrice(ctx, priceChan)
	//if err != nil {
	//	log.Fatalf("Ошибка подписки на цены: %v", err)
	//}

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
				break
			default:
				// Если нет новых цен, просто ждём
				log.Println("Нет новых цен...")
			}

		}
	}()

	// Ждём сигнала завершения
	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем...")
	cancel()
}
