package storage

import (
	"sync"
)

// MemoryStorage реализует хранилище в памяти
type MemoryStorage struct {
	limits map[string]ClientLimit
	mutex  sync.RWMutex
}

// NewMemoryStorage создает новое хранилище в памяти
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		limits: make(map[string]ClientLimit),
	}
}

// SaveClientLimit сохраняет настройки лимита для клиента
func (s *MemoryStorage) SaveClientLimit(clientID string, capacity int, refillRate float64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.limits[clientID] = ClientLimit{
		Capacity:   capacity,
		RefillRate: refillRate,
	}

	return nil
}

// GetClientLimit получает настройки лимита для клиента
func (s *MemoryStorage) GetClientLimit(clientID string) (capacity int, refillRate float64, exists bool, err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	limit, exists := s.limits[clientID]
	if !exists {
		return 0, 0, false, nil
	}

	return limit.Capacity, limit.RefillRate, true, nil
}

// LoadAllClientLimits загружает все настройки лимитов
func (s *MemoryStorage) LoadAllClientLimits() (map[string]ClientLimit, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	limits := make(map[string]ClientLimit, len(s.limits))
	for id, limit := range s.limits {
		limits[id] = limit
	}

	return limits, nil
}

// DeleteClientLimit удаляет настройки лимита для клиента
func (s *MemoryStorage) DeleteClientLimit(clientID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.limits, clientID)
	return nil
}

// Close закрывает хранилище
func (s *MemoryStorage) Close() error {
	return nil
}
