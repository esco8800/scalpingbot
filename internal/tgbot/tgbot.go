package tgbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"net/http"
	"scalpingbot/internal/buffer"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"scalpingbot/internal/tools"
	"scalpingbot/internal/workers/sell_v1"
	"strings"
	"time"
)

const (
	WorkerStatusKey = "worker_status"

	start_worker = "start_worker"
	stop_worker  = "stop_worker"
	logs         = "logs"
	stats        = "stats"
)

type TelegramBot struct {
	bot     *tgbotapi.BotAPI
	token   string
	chatID  int64
	ringBuf buffer.Buffer
	storage repo.Repo
	ex      exchange.Exchange
	cfg     config.Config
}
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type TelegramUpdate struct {
	Message struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

func NewTelegramBot(cfg config.Config, ringBuf buffer.Buffer, storage repo.Repo, ex exchange.Exchange) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TgToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	tgBot := &TelegramBot{
		bot:     bot,
		token:   cfg.TgToken,
		chatID:  cfg.TgChatID,
		ringBuf: ringBuf,
		storage: storage,
		ex:      ex,
		cfg:     cfg,
	}

	// Регистрируем команды
	if err := tgBot.registerCommands(); err != nil {
		log.Printf("Ошибка регистрации команд: %v", err)
		return tgBot, err
	}

	return tgBot, nil
}

// Start запускает обработку команд бота
func (tb *TelegramBot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tb.bot.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			if update.Message == nil || !update.Message.IsCommand() {
				continue
			}

			if err := tb.handleCommand(update.Message); err != nil {
				log.Printf("Error handling command: %v", err)
				tb.sendMessage("Error processing command")
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// handleCommand обрабатывает входящие команды
func (tb *TelegramBot) handleCommand(msg *tgbotapi.Message) error {
	var message string

	switch msg.Command() {
	case start_worker:
		message = "Worker started"
		tb.storage.Add(WorkerStatusKey)
	case stop_worker:
		message = "Worker stopped"
		tb.storage.Remove(WorkerStatusKey)
	case logs:
		message = "Last messages:\n" + tb.ringBuf.GetMessages()
	case stats:
		openOrders, err := tb.ex.GetOpenOrders(context.Background(), tb.cfg.Symbol)
		if err != nil {
			return err
		}
		buyCount, sellCount := sell_v1.GetCountOpenOrders(openOrders)

		now := time.Now()
		endTime := now.UnixMilli()
		// Получаем первый день текущего месяца
		startTime := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).UnixMilli()

		allOrders, err := tb.ex.GetAllOrders(context.Background(), tb.cfg.Symbol, startTime, endTime)
		if err != nil {
			return err
		}

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("Count of open Buy Orders: %d \n", buyCount))
		builder.WriteString(fmt.Sprintf("Count of open Sell Orders: %d \n", sellCount))
		builder.WriteString(fmt.Sprintf("Total Profit this month: %.3f \n", tools.CalculateSellVolumeInUSDT(allOrders)))

		message = builder.String()
	default:
		message = "Неизвестная команда"
	}

	return tb.sendMessage(message)
}

// sendMessage отправляет сообщение в чат
func (tb *TelegramBot) sendMessage(text string) error {
	msg := tgbotapi.NewMessage(tb.chatID, text)
	_, err := tb.bot.Send(msg)
	return err
}

// registerCommands - регистрация команд
func (tb *TelegramBot) registerCommands() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands", tb.token)

	commands := []BotCommand{
		{Command: start_worker, Description: "Запустить воркер"},
		{Command: stop_worker, Description: "Остановить воркер"},
		{Command: logs, Description: "Получить последние логи"},
		{Command: stats, Description: "Получить cтатистику работы"},
	}

	payload, err := json.Marshal(map[string][]BotCommand{"commands": commands})
	if err != nil {
		return fmt.Errorf("ошибка формирования JSON: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("ошибка отправки в Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неуспешный ответ от Telegram API: %s", resp.Status)
	}

	log.Println("Команды успешно зарегистрированы в Telegram")
	return nil
}
