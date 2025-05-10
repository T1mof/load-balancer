package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStorage реализует хранилище на основе PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// Config содержит настройки подключения к PostgreSQL
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewPostgresStorage создает новое хранилище PostgreSQL
func NewPostgresStorage(config Config) (*PostgresStorage, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к PostgreSQL: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка проверки соединения с PostgreSQL: %w", err)
	}

	storage := &PostgresStorage{db: db}

	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка инициализации схемы БД: %w", err)
	}

	return storage, nil
}

// Close закрывает соединение с БД
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

// initSchema создает необходимые таблицы, если они не существуют
func (s *PostgresStorage) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS rate_limits (
			client_id VARCHAR(255) PRIMARY KEY,
			capacity INTEGER NOT NULL,
			refill_rate FLOAT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

// SaveClientLimit сохраняет настройки лимита для клиента
func (s *PostgresStorage) SaveClientLimit(clientID string, capacity int, refillRate float64) error {
	_, err := s.db.Exec(`
		INSERT INTO rate_limits (client_id, capacity, refill_rate, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (client_id) 
		DO UPDATE SET 
			capacity = $2,
			refill_rate = $3,
			updated_at = NOW()
	`, clientID, capacity, refillRate)

	if err != nil {
		return fmt.Errorf("ошибка сохранения лимита: %w", err)
	}
	return nil
}

// GetClientLimit получает настройки лимита для клиента
func (s *PostgresStorage) GetClientLimit(clientID string) (capacity int, refillRate float64, exists bool, err error) {
	row := s.db.QueryRow(`
		SELECT capacity, refill_rate FROM rate_limits
		WHERE client_id = $1
	`, clientID)

	err = row.Scan(&capacity, &refillRate)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("ошибка получения лимита: %w", err)
	}
	return capacity, refillRate, true, nil
}

// LoadAllClientLimits загружает все настройки лимитов
func (s *PostgresStorage) LoadAllClientLimits() (map[string]ClientLimit, error) {
	rows, err := s.db.Query(`
		SELECT client_id, capacity, refill_rate FROM rate_limits
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки лимитов: %w", err)
	}
	defer rows.Close()

	limits := make(map[string]ClientLimit)
	for rows.Next() {
		var clientID string
		var capacity int
		var refillRate float64
		if err := rows.Scan(&clientID, &capacity, &refillRate); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		limits[clientID] = ClientLimit{
			Capacity:   capacity,
			RefillRate: refillRate,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов: %w", err)
	}
	return limits, nil
}

// DeleteClientLimit удаляет настройки лимита для клиента
func (s *PostgresStorage) DeleteClientLimit(clientID string) error {
	_, err := s.db.Exec(`
		DELETE FROM rate_limits WHERE client_id = $1
	`, clientID)

	if err != nil {
		return fmt.Errorf("ошибка удаления лимита: %w", err)
	}
	return nil
}
