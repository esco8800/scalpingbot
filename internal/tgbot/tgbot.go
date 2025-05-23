package tgbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"
	"log"
	"net/http"
	"regexp"
	"scalpingbot/internal/buffer"
	"scalpingbot/internal/config"
	"scalpingbot/internal/exchange"
	"scalpingbot/internal/repo"
	"scalpingbot/internal/repository"
	"scalpingbot/internal/workers/sell_v1"
	"strconv"
	"strings"
	"time"
)

var allowedUsers = map[int64]bool{
	280019658: true,
}

const (
	WorkerStatusKey = "worker_status"

	start_worker = "start_worker"
	stop_worker  = "stop_worker"
	logs         = "logs"
	stats        = "stats"
	start        = "start"
	set_settings = "set_settings"
)

type TelegramBot struct {
	bot           *tgbotapi.BotAPI
	token         string
	chatID        int64
	ringBuf       buffer.Buffer
	storage       repo.Repo
	ex            exchange.Exchange
	cfg           config.Config
	profitStorage repo.ProfitRepo
	sqlLiteDb     repository.UserRepository
	limiter       *rate.Limiter
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

func NewTelegramBot(cfg config.Config, ringBuf buffer.Buffer, storage repo.Repo, ex exchange.Exchange, profitStorage repo.ProfitRepo, sqlLiteDb repository.UserRepository) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TgToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	tgBot := &TelegramBot{
		bot:           bot,
		token:         cfg.TgToken,
		chatID:        cfg.TgChatID,
		ringBuf:       ringBuf,
		storage:       storage,
		ex:            ex,
		cfg:           cfg,
		profitStorage: profitStorage,
		sqlLiteDb:     sqlLiteDb,
		limiter:       rate.NewLimiter(rate.Every(time.Second), 1), // 1 команда в секунду
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
	if !tb.limiter.Allow() {
		return tb.sendMessage("Too many requests, please try again later")
	}
	if !allowedUsers[msg.From.ID] {
		return tb.sendMessage("You are not allowed to use this bot")
	}

	var message string

	switch msg.Command() {
	case start:
		message = "Бот для автоскальпинга. Подробное описание тут - (в разработке)\n" +
			"Bot for auto-scalping. Detailed description here - (in development)"
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

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("Count of open Buy Orders: %d \n", buyCount))
		builder.WriteString(fmt.Sprintf("Count of open Sell Orders: %d \n", sellCount))

		profit, ok := tb.profitStorage.Get(repo.ProfitKey)
		if !ok {
			builder.WriteString("Total Profit last 7d: calculating...\n")
		} else {
			builder.WriteString(fmt.Sprintf("Total Profit last 7d: %.3f USDT\n", profit))
		}

		message = builder.String()
	case set_settings:
		err := tb.handleSetSettings(msg)
		if err != nil {
			return err
		}
		message = "Settings updated successfully"
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
		{Command: start, Description: "What can this bot do"},
		{Command: start_worker, Description: "Start worker"},
		{Command: stop_worker, Description: "Stop worker"},
		{Command: logs, Description: "Get last log messages"},
		{Command: stats, Description: "Get stats"},
		{Command: set_settings, Description: "Set user settings (profit_percent, order_size, base_buy_timeout, api_key, secret_key, symbol)"},
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

// handleSetSettings обрабатывает команду установки настроек пользователя
func (tb *TelegramBot) handleSetSettings(msg *tgbotapi.Message) error {
	args := strings.Fields(msg.Text)
	if len(args) != 7 {
		return tb.sendMessage("Usage: /setsettings <profit_percent> <order_size> <base_buy_timeout> <api_key> <secret_key> <symbol>")
	}

	profitPercent, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return tb.sendMessage("Invalid profit_percent format")
	}
	if profitPercent < 0 {
		return tb.sendMessage("Profit percent must be non-negative")
	}

	orderSize, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return tb.sendMessage("Invalid order_size format")
	}
	if orderSize <= 0 {
		return tb.sendMessage("Order size must be positive")
	}

	// Проверяем, что profitPercent не превышают разумные пределы
	if profitPercent > 10 {
		return tb.sendMessage("Profit percent must be <= 10")
	}

	baseBuyTimeout, err := strconv.Atoi(args[3])
	if err != nil {
		return tb.sendMessage("Invalid base_buy_timeout format")
	}
	if baseBuyTimeout <= 0 {
		return tb.sendMessage("Base buy timeout must be positive")
	}

	apiKey := args[4]
	if apiKey == "" {
		return tb.sendMessage("API key cannot be empty")
	}

	secretKey := args[5]
	if secretKey == "" {
		return tb.sendMessage("Secret key cannot be empty")
	}

	symbol := args[6]
	if symbol == "" {
		return tb.sendMessage("Symbol cannot be empty")
	}

	// Проверяем длину строк
	if len(apiKey) > 256 || len(secretKey) > 256 || len(symbol) > 20 {
		return tb.sendMessage("Input values are too long")
	}

	// Проверяем формат символа
	if !regexp.MustCompile(`^[A-Z0-9]+$`).MatchString(symbol) {
		return tb.sendMessage("Symbol must contain only uppercase letters and numbers")
	}

	user := repository.User{
		TelegramID:     fmt.Sprintf("%d", msg.From.ID),
		Username:       msg.From.UserName,
		ProfitPercent:  profitPercent,
		OrderSize:      orderSize,
		BaseBuyTimeout: baseBuyTimeout,
		APIKey:         apiKey,
		SecretKey:      secretKey,
		Symbol:         symbol,
	}

	// Проверяем, существует ли пользователь
	_, err = tb.sqlLiteDb.GetUserByID(context.Background(), user.TelegramID)
	if err != nil {
		if err.Error() == "user not found" {
			// Создаем нового пользователя
			err = tb.sqlLiteDb.CreateUser(context.Background(), user)
			if err != nil {
				return tb.sendMessage(fmt.Sprintf("Failed to create user: %v", err))
			}
		} else {
			return tb.sendMessage(fmt.Sprintf("Error checking user: %v", err))
		}
	} else {
		// Обновляем существующего пользователя
		err = tb.sqlLiteDb.UpdateUser(context.Background(), user)
		if err != nil {
			return tb.sendMessage(fmt.Sprintf("Failed to update user: %v", err))
		}
	}

	return nil
}
