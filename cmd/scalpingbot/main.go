package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"scalpingbot/internal/buffer"
	"scalpingbot/internal/listener"
	"scalpingbot/internal/logger"
	"scalpingbot/internal/repository"
	"scalpingbot/internal/tgbot"
	"scalpingbot/internal/workers/profit_calc"
	"syscall"

	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"scalpingbot/internal/worker"
	"scalpingbot/internal/workers/buy_v1"
	"scalpingbot/internal/workers/sell_v1"
	"time"
)

func main() {
	// Перенаправляем вывод в консоль и в буфер (чтобы потом отправить в телеграм)
	ringBuffer := buffer.NewRingBuffer(10)
	multiWriter := io.MultiWriter(os.Stdout, ringBuffer)
	log.SetOutput(multiWriter)

	// Создаём контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Загружаем конфигурацию через Viper
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}
	// Создаём репозитории для хранения данных
	storage := repo.NewSafeSet()
	profitStorage := repo.NewSProfitStorage()

	sqlLiteDb, err := repository.NewSQLiteUserRepository(cfg.DbPath)
	if err != nil {
		log.Fatalf("Ошибка создания SQLite репозитория: %v", err)
	}

	logLoger := logger.SetupLogger(cfg.TgToken, cfg.TgChatID)

	// Создаём клиента MEXC и сторедж
	ex := exchange.NewMEXCClient(cfg.APIKey, cfg.SecretKey, cfg.Symbol, logLoger)

	// Инициализация Telegram бота
	bot, err := tgbot.NewTelegramBot(cfg, ringBuffer, storage, ex, profitStorage)
	if err != nil {
		log.Fatalf("Ошибка инициализации Telegram бота: %v", err)
	}
	// Запускаем тг бота в отдельной горутине для неблокирующего вызова
	go func() {
		if err := bot.Start(ctx); err != nil {
			log.Fatalf("Bot stopped with error: %v", err)
		}
	}()

	// Инициализируем и запускаем воркеры
	log.Println("Запуск воркеров...")
	buyWorker := buy_v1.NewBot(cfg, ex, storage)
	err = worker.Start(ctx, buyWorker, time.Second*5, logLoger)
	if err != nil {
		log.Fatalf("Ошибка запуска buyWorker: %v", err)
	}
	sellWorker := sell_v1.NewBot(cfg, ex, storage)
	err = worker.Start(ctx, sellWorker, time.Minute, logLoger)
	if err != nil {
		log.Fatalf("Ошибка запуска sellWorker: %v", err)
	}
	profitWorker := profit_calc.NewBot(cfg, ex, profitStorage)
	err = worker.Start(ctx, profitWorker, 30*time.Minute, logLoger)
	if err != nil {
		log.Fatalf("Ошибка запуска profitWorker: %v", err)
	}

	log.Println("Запуск подписки на обновления ордеров...")
	updateCh := make(chan exchange.OrderUpdate, 100)
	err = ex.SubscribeOrderUpdates(ctx, updateCh)
	if err != nil {
		log.Fatalf("Ошибка подписки на обновления ордеров.: %v", err)
	}

	log.Println("Запуск лиснера ордеров...")
	orderListener := listener.NewOrderListener(cfg, ex, updateCh, logLoger, storage)
	orderListener.Start(ctx)

	// Настраиваем graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем бота...")
	cancel()
	log.Println("Бот остановлен")
}
