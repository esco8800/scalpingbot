package logger

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
)

type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)
}

type TelegramLogger struct {
	Token  string
	ChatID string
}

func NewTelegramLogger(token, chatID string) *TelegramLogger {
	return &TelegramLogger{
		Token:  token,
		ChatID: chatID,
	}
}

func (t *TelegramLogger) SendMessage(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	body := fmt.Sprintf(`{"chat_id": "%s", "text": "%s"}`, t.ChatID, message)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error: %s", resp.Status)
	}

	return nil
}

// logHook - хук для Logrus
type logHook struct {
	Telegram *TelegramLogger
}

func (h *logHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel}
}

func (h *logHook) Fire(entry *logrus.Entry) error {
	message := fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Message)
	return h.Telegram.SendMessage(message)
}

// logrusLogger - основная реализация логгера
type logrusLogger struct {
	logger   *logrus.Logger
	telegram *TelegramLogger
}

func SetupLogger(token, chatID string) Logger {
	tgLogger := NewTelegramLogger(token, chatID)
	log := logrus.New()

	// Лог в консоль
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	// Хук для Telegram (только Error и Fatal)
	log.AddHook(&logHook{Telegram: tgLogger})

	return &logrusLogger{
		logger:   log,
		telegram: tgLogger,
	}
}

// Implementing Logger interface methods
func (l *logrusLogger) Info(msg string) {
	l.logger.Info(msg)
}

func (l *logrusLogger) Warn(msg string) {
	l.logger.Warn(msg)
}

func (l *logrusLogger) Error(msg string) {
	l.logger.Error(msg)
}

func (l *logrusLogger) Fatal(msg string) {
	l.logger.Fatal(msg)
}
