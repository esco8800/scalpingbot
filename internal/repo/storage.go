package repo

import (
	"sync"
)

type Repo interface {
	Add(key string)
	Remove(key string)
	Has(key string) bool
}

// SafeSet — потокобезопасный набор строк
type SafeSet struct {
	mu    sync.RWMutex
	items map[string]struct{}
}

// NewSafeSet — создать новый SafeSet
func NewSafeSet() *SafeSet {
	return &SafeSet{
		items: make(map[string]struct{}),
	}
}

// Add — добавить элемент в кеш
func (s *SafeSet) Add(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = struct{}{}
}

// Remove — удалить элемент из кеша
func (s *SafeSet) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// Has — проверить наличие элемента
func (s *SafeSet) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.items[key]
	return exists
}

// Len — получить количество элементов в кеше
func (s *SafeSet) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Clear — очистить весь кеш
func (s *SafeSet) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]struct{})
}
