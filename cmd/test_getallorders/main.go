package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"scalpingbot/internal/logger"
	"syscall"
	"time"

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

	// Создаём клиента MEXC и сторедж
	ex := exchange.NewMEXCClient(cfg.APIKey, cfg.SecretKey, cfg.Symbol, logLoger)

	// Создаём контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()
	endTime := now.UnixMilli()
	startTime := now.Add(-59 * 24 * 7 * time.Minute).UnixMilli()

	allOrders, err := ex.GetAllOrders(context.Background(), cfg.Symbol, startTime, endTime)
	if err != nil {
		log.Fatalf("Ошибка получения всех ордеров: %v", err)
	}
	log.Printf("Время первого оредра %d", allOrders[0].Time)
	log.Printf("Врем последнего ордера %d", allOrders[len(allOrders)-1].Time)

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
