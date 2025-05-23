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
	"scalpingbot/internal/tools"
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

	// Получаем klines
	klines, err := ex.GetKlines(ctx, cfg.Symbol, exchange.KlineInterval1m, 10)
	if err != nil {
		log.Fatalf("Ошибка получения klines: %v", err)
	}
	log.Printf("Klines len: %d", len(klines))

	// Рассчитываем таймаут
	timeout := tools.AdjustTimeout(cfg.BaseBuyTimeout, klines)
	log.Printf("Timeout: %d", timeout)

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ждём сигнала завершения
	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем...")
	cancel()
}
