package repo

import (
	"sync"
)

const ProfitKey = "profit"

type ProfitRepo interface {
	Add(key string, value float64)
	Remove(key string)
	Get(key string) (float64, bool)
}

type ProfitStorage struct {
	mu    sync.RWMutex
	items map[string]float64
}

func NewSProfitStorage() *ProfitStorage {
	return &ProfitStorage{
		items: make(map[string]float64),
	}
}

// Add — добавить элемент в кеш
func (s *ProfitStorage) Add(key string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
}

// Remove — удалить элемент из кеша
func (s *ProfitStorage) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

func (s *ProfitStorage) Get(key string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.items[key]
	return value, exists
}

// Clear — очистить весь кеш
func (s *ProfitStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]float64)
}
