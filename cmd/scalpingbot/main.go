package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"scalpingbot/internal/bot"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
)

func main() {
	// Загружаем конфигурацию через Viper
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создаём клиента MEXC
	ex := exchange.NewMEXCExchange(cfg.APIKey, cfg.SecretKey, cfg.Symbol)

	// Создаём контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем и запускаем бота
	b := bot.NewBot(cfg, ex)
	log.Println("Запуск бота...")
	go b.Start(ctx)

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем бота...")
	cancel()
	log.Println("Бот остановлен")
}
