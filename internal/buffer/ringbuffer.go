package buffer

import (
	"strings"
	"sync"
)

type Buffer interface {
	GetMessages() string
}

type RingBuffer struct {
	mu       sync.Mutex
	messages []string
	index    int
	full     bool
	max      int
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		messages: make([]string, size),
		max:      size,
	}
}

// Write - потокобезопасная запись сообщения в буфер
func (rb *RingBuffer) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))

	if message != "" {
		rb.mu.Lock()
		defer rb.mu.Unlock()

		rb.messages[rb.index] = message
		rb.index = (rb.index + 1) % rb.max

		if rb.index == 0 {
			rb.full = true
		}
	}

	return len(p), nil
}

// GetMessages - потокобезопасное получение сообщений из буфера
func (rb *RingBuffer) GetMessages() string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var result []string

	if rb.full {
		result = append(result, rb.messages[rb.index:]...)
	}

	result = append(result, rb.messages[:rb.index]...)

	return strings.Join(result, "\n")
}
