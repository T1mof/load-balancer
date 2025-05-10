package storage

// ClientLimit структура для хранения настроек лимита
type ClientLimit struct {
	Capacity   int
	RefillRate float64
}

// Storage интерфейс для хранения настроек
type Storage interface {
	SaveClientLimit(clientID string, capacity int, refillRate float64) error
	GetClientLimit(clientID string) (capacity int, refillRate float64, exists bool, err error)
	LoadAllClientLimits() (map[string]ClientLimit, error)
	DeleteClientLimit(clientID string) error
	Close() error
}
