package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"scalpingbot/internal/workers/buy_v1"
	"scalpingbot/internal/workers/sell_v1"
	"scalpingbot/internal/worker"
	"time"
)

func main() {
	// Создаём контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Загружаем конфигурацию через Viper
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создаём клиента MEXC и сторедж
	ex := exchange.NewMEXCClient(cfg.APIKey, cfg.SecretKey, cfg.Symbol)
	storage := repo.NewSafeSet()

	// Инициализируем и запускаем воркеры
	log.Println("Запуск воркеров...")
	buyWorker := buy_v1.NewBot(cfg, ex, storage)
	err = worker.Start(ctx, buyWorker, time.Second*5)
	if err != nil {
		log.Fatalf("Ошибка запуска buyWorker: %v", err)
	}
	sellWorker := sell_v1.NewBot(cfg, ex, storage)
	err = worker.Start(ctx, sellWorker, time.Second*5)
	if err != nil {
		log.Fatalf("Ошибка запуска sellWorker: %v", err)
	}

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем бота...")
	cancel()
	log.Println("Бот остановлен")
}
