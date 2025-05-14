package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"scalpingbot/internal/listener"
	"scalpingbot/internal/logger"
	"scalpingbot/internal/repo"
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
	logLoger := logger.SetupLogger(cfg.TgToken, cfg.TgChatID)
	ex := exchange.NewMEXCClient(cfg.APIKey, cfg.SecretKey, cfg.Symbol, logLoger)

	// Создаём контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Запуск подписки на обновления ордеров...")
	updateCh := make(chan exchange.OrderUpdate, 100)
	err = ex.SubscribeOrderUpdates(ctx, updateCh)
	if err != nil {
		log.Fatalf("Ошибка подписки на обновления ордеров.: %v", err)
	}

	log.Println("Запуск лиснера ордеров...")
	orderListener := listener.NewOrderListener(cfg, ex, updateCh, logLoger, repo.NewSafeSet())
	orderListener.Start(ctx)

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Основной цикл вывода
	go func() {
		for {
			select {
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
