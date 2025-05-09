package tgbot

import (
	"fmt"
	"log"
	"net/http"
	"scalpingbot/internal/buffer"
	"scalpingbot/internal/repo"
	"strings"
)

const WorkerStatusKey = "worker_status"

type TelegramBot struct {
	token   string
	chatID  string
	ringBuf buffer.Buffer
	storage repo.Repo
}

func NewTelegramBot(token, chatID string, ringBuf buffer.Buffer, storage repo.Repo) *TelegramBot {
	return &TelegramBot{
		token:   token,
		chatID:  chatID,
		ringBuf: ringBuf,
		storage: storage,
	}
}

func (tb *TelegramBot) Start() {
	http.HandleFunc("/"+tb.token, tb.handleWebhook)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (tb *TelegramBot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var message string
	text := r.URL.Query().Get("text")

	switch {
	case strings.HasPrefix(text, "/start_worker"):
		message = "Воркер запущен"
		// Запуск воркера
		tb.storage.Add(WorkerStatusKey)
	case strings.HasPrefix(text, "/stop_worker"):
		message = "Воркер остановлен"
		// Остановка воркера
		tb.storage.Remove(WorkerStatusKey)
	case strings.HasPrefix(text, "/logs"):
		message = "Последние сообщения:\n" + tb.ringBuf.GetMessages()
	default:
		message = "Неизвестная команда"
	}

	tb.sendMessage(message)
}

func (tb *TelegramBot) sendMessage(text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s&text=%s", tb.token, tb.chatID, text)
	http.Get(url)
}
