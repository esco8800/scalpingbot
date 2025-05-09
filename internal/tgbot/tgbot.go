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
	"scalpingbot/internal/repo"
)

const (
	WorkerStatusKey = "worker_status"

	start_worker = "start_worker"
	stop_worker  = "stop_worker"
	logs         = "logs"
)

type TelegramBot struct {
	bot     *tgbotapi.BotAPI
	token   string
	chatID  int64
	ringBuf buffer.Buffer
	storage repo.Repo
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

func NewTelegramBot(token string, chatID int64, ringBuf buffer.Buffer, storage repo.Repo) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	tgBot := &TelegramBot{
		bot:     bot,
		token:   token,
		chatID:  chatID,
		ringBuf: ringBuf,
		storage: storage,
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
		message = "Воркер запущен"
		tb.storage.Add(WorkerStatusKey)
	case stop_worker:
		message = "Воркер остановлен"
		tb.storage.Remove(WorkerStatusKey)
	case logs:
		message = "Последние сообщения:\n" + tb.ringBuf.GetMessages()
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
