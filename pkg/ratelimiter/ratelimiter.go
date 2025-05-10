package ratelimiter

import (
	"load-balancer/pkg/storage"
	"net/http"
	"sync"
	"time"
)

// Logger интерфейс для логирования
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// TokenBucket представляет ведро токенов для отдельного клиента
type TokenBucket struct {
	capacity   int       // Максимальное количество токенов
	tokens     int       // Текущее количество токенов
	refillRate float64   // Токенов в секунду
	lastRefill time.Time // Время последнего пополнения
	lastAccess time.Time // Время последнего доступа (для очистки неактивных)
	mutex      sync.Mutex
}

// RateLimiter управляет ограничением запросов для всех клиентов
type RateLimiter struct {
	buckets     map[string]*TokenBucket // Ведра токенов по IP/API-ключу
	defaultCap  int                     // Емкость по умолчанию
	defaultRate float64                 // Скорость пополнения по умолчанию
	logger      Logger
	storage     storage.Storage // Хранилище настроек
	mutex       sync.RWMutex
}

// NewRateLimiter создает экземпляр ограничителя запросов
func NewRateLimiter(defaultCap int, defaultRate float64, logger Logger, storage storage.Storage) *RateLimiter {
	limiter := &RateLimiter{
		buckets:     make(map[string]*TokenBucket),
		defaultCap:  defaultCap,
		defaultRate: defaultRate,
		logger:      logger,
		storage:     storage,
	}

	// Загружаем настройки из хранилища
	if storage != nil {
		limiter.loadLimitsFromStorage()
	}

	// Запускаем периодическое обновление токенов
	go limiter.refillTokens()

	// Запускаем периодическую очистку неактивных buckets
	go limiter.cleanupInactiveBuckets()

	return limiter
}

// loadLimitsFromStorage загружает настройки клиентов из хранилища
func (rl *RateLimiter) loadLimitsFromStorage() {
	clientLimits, err := rl.storage.LoadAllClientLimits()
	if err != nil {
		rl.logger.Errorf("Не удалось загрузить настройки лимитов из хранилища: %v", err)
		return
	}

	for clientID, limit := range clientLimits {
		rl.createBucketInMemory(clientID, limit.Capacity, limit.RefillRate)
		rl.logger.Debugf("Загружены настройки для клиента %s из хранилища: capacity=%d, rate=%.2f",
			clientID, limit.Capacity, limit.RefillRate)
	}
}

// createBucketInMemory создает новый бакет в памяти без обращения к хранилищу
func (rl *RateLimiter) createBucketInMemory(clientID string, capacity int, refillRate float64) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.buckets[clientID] = &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: now,
		lastAccess: now,
	}
}

// Allow проверяет, допустим ли запрос от клиента
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.logger.Infof("Проверка лимита для клиента: %s", clientID)

	bucket := rl.getBucket(clientID)

	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()

	// Пополняем токены с учетом прошедшего времени
	rl.refillBucket(bucket)

	// Отладочная информация
	rl.logger.Debugf("Клиент: %s, емкость: %d, текущие токены: %d",
		clientID, bucket.capacity, bucket.tokens)

	// Обновляем время последнего доступа
	bucket.lastAccess = time.Now()

	if bucket.tokens > 0 {
		bucket.tokens--
		rl.logger.Infof("Запрос разрешен для клиента %s (осталось токенов: %d)",
			clientID, bucket.tokens)
		return true
	}

	rl.logger.Infof("Запрос отклонен для клиента %s (нет токенов)", clientID)
	return false
}

// getBucket возвращает ведро для клиента (создает новое, если нужно)
func (rl *RateLimiter) getBucket(clientID string) *TokenBucket {
	// Сначала проверяем без блокировки на запись
	rl.mutex.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mutex.RUnlock()

	if exists {
		return bucket
	}

	// Если ведра нет, проверяем настройки в хранилище
	var capacity int = rl.defaultCap
	var refillRate float64 = rl.defaultRate
	var storedSettings bool = false

	if rl.storage != nil {
		storedCapacity, storedRate, exists, err := rl.storage.GetClientLimit(clientID)
		if err != nil {
			rl.logger.Warnf("Ошибка при получении настроек из хранилища для %s: %v", clientID, err)
		} else if exists {
			capacity = storedCapacity
			refillRate = storedRate
			storedSettings = true
		}
	}

	// Создаем новое ведро (блокировка на запись)
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// Проверяем еще раз после получения эксклюзивной блокировки
	bucket, exists = rl.buckets[clientID]
	if exists {
		return bucket
	}

	// Создаем новое ведро
	now := time.Now()
	bucket = &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: now,
		lastAccess: now,
	}

	rl.buckets[clientID] = bucket

	// Если не нашли настройки в хранилище, сохраняем дефолтные
	if rl.storage != nil && !storedSettings {
		go func() {
			if err := rl.storage.SaveClientLimit(clientID, capacity, refillRate); err != nil {
				rl.logger.Warnf("Не удалось сохранить дефолтные настройки лимита для клиента %s: %v", clientID, err)
			}
		}()
	}

	rl.logger.Infof("Создано новое ведро токенов для клиента %s", clientID)
	return bucket
}

// refillBucket пополняет токены в ведре с учетом прошедшего времени
func (rl *RateLimiter) refillBucket(bucket *TokenBucket) {
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()

	// Добавляем токены согласно скорости пополнения
	newTokens := int(elapsed * bucket.refillRate)
	if newTokens > 0 {
		bucket.tokens += newTokens
		if bucket.tokens > bucket.capacity {
			bucket.tokens = bucket.capacity
		}
		bucket.lastRefill = now
	}
}

// refillTokens периодически обновляет токены для всех клиентов
func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		rl.mutex.RLock()
		for _, bucket := range rl.buckets {
			bucket.mutex.Lock()
			rl.refillBucket(bucket)
			bucket.mutex.Unlock()
		}
		rl.mutex.RUnlock()
	}
}

// cleanupInactiveBuckets периодически удаляет неактивные buckets
func (rl *RateLimiter) cleanupInactiveBuckets() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		now := time.Now()
		inactiveThreshold := 30 * time.Minute

		rl.mutex.Lock()
		for clientID, bucket := range rl.buckets {
			bucket.mutex.Lock()
			inactive := now.Sub(bucket.lastAccess) > inactiveThreshold
			bucket.mutex.Unlock()

			if inactive {
				delete(rl.buckets, clientID)
				rl.logger.Infof("Удален неактивный bucket для клиента %s", clientID)
			}
		}
		rl.mutex.Unlock()
	}
}

// SetClientLimit устанавливает индивидуальные настройки лимита для клиента
func (rl *RateLimiter) SetClientLimit(clientID string, capacity int, refillRate float64) {
	// Обновляем бакет в памяти
	rl.mutex.Lock()
	bucket, exists := rl.buckets[clientID]
	if exists {
		bucket.mutex.Lock()
		bucket.capacity = capacity
		bucket.refillRate = refillRate
		if bucket.tokens > capacity {
			bucket.tokens = capacity
		}
		bucket.mutex.Unlock()
	} else {
		now := time.Now()
		rl.buckets[clientID] = &TokenBucket{
			capacity:   capacity,
			tokens:     capacity,
			refillRate: refillRate,
			lastRefill: now,
			lastAccess: now,
		}
	}
	rl.mutex.Unlock()

	// Сохраняем настройки в хранилище, если оно доступно
	if rl.storage != nil {
		if err := rl.storage.SaveClientLimit(clientID, capacity, refillRate); err != nil {
			rl.logger.Errorf("Не удалось сохранить настройки лимита для клиента %s: %v", clientID, err)
			return
		}
	}

	rl.logger.Infof("Установлен лимит для клиента %s: capacity=%d, rate=%.2f", clientID, capacity, refillRate)
}

// GetClientLimit возвращает текущие настройки лимита для клиента
func (rl *RateLimiter) GetClientLimit(clientID string) (capacity int, refillRate float64, exists bool) {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	bucket, exists := rl.buckets[clientID]
	if !exists {
		return rl.defaultCap, rl.defaultRate, false
	}

	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	return bucket.capacity, bucket.refillRate, true
}

// DeleteClientLimit удаляет настройки лимита для клиента
func (rl *RateLimiter) DeleteClientLimit(clientID string) error {
	// Удаляем из памяти
	rl.mutex.Lock()
	delete(rl.buckets, clientID)
	rl.mutex.Unlock()

	// Удаляем из хранилища, если оно доступно
	if rl.storage != nil {
		if err := rl.storage.DeleteClientLimit(clientID); err != nil {
			rl.logger.Errorf("Не удалось удалить настройки лимита для клиента %s из хранилища: %v", clientID, err)
			return err
		}
	}

	rl.logger.Infof("Удалены настройки лимита для клиента %s", clientID)
	return nil
}

// GetAllClients возвращает список всех клиентов
func (rl *RateLimiter) GetAllClients() []ClientLimitResponse {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	clients := make([]ClientLimitResponse, 0, len(rl.buckets))

	for clientID, bucket := range rl.buckets {
		bucket.mutex.Lock()
		clients = append(clients, ClientLimitResponse{
			ClientID:   clientID,
			Capacity:   bucket.capacity,
			RefillRate: bucket.refillRate,
		})
		bucket.mutex.Unlock()
	}

	return clients
}

// RateLimitMiddleware возвращает middleware для ограничения запросов
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)
			limiter.logger.Debugf("Обработка запроса от клиента: %s", clientID)

			if !limiter.Allow(clientID) {
				limiter.logger.Warnf("Превышен лимит запросов для клиента %s", clientID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"code": 429, "message": "Rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientID определяет идентификатор клиента на основе API-ключа или IP-адреса
func getClientID(r *http.Request) string {
	// Прямое использование заголовка X-API-Key без добавления префикса
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		// Убираем префикс "api:" - тесты ожидают точное совпадение с clientID
		return apiKey
	}

	// Если API-ключ отсутствует, используем IP-адрес
	return "ip:" + r.RemoteAddr
}
