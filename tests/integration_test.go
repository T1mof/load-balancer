package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL        = "http://loadbalancer:8080"
	parallelCount  = 10
	requestsPerGo  = 5
	testClientBase = "test_client"
)

// Структуры для работы с API
type ClientLimitRequest struct {
	Capacity   int     `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
}

type ClientLimitResponse struct {
	ClientID   string  `json:"client_id"`
	Capacity   int     `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
	Message    string  `json:"message,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Тестовый клиент для обращения к API
type TestClient struct {
	httpClient *http.Client
}

// Структура для интеграционных тестов
type IntegrationTest struct {
	client   *TestClient
	pgDB     *sql.DB
	testName string
}

// Настройка тестового окружения
func setupTest(t *testing.T) *IntegrationTest {
	t.Helper()

	client := &TestClient{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	db, err := sql.Open("postgres", "host=postgres port=5432 user=postgres password=secret dbname=loadbalancer sslmode=disable")
	if err != nil {
		t.Logf("Не удалось подключиться к PostgreSQL: %v", err)
	} else {
		_, err = db.Exec("DELETE FROM rate_limits WHERE client_id LIKE 'test_%'")
		if err != nil {
			t.Logf("Ошибка при очистке данных: %v", err)
		}
	}

	return &IntegrationTest{
		client:   client,
		pgDB:     db,
		testName: t.Name(),
	}
}

// Завершение теста и очистка
func (it *IntegrationTest) cleanup(t *testing.T) {
	t.Helper()
	if it.pgDB != nil {
		it.pgDB.Close()
	}
}

// Утилитарные методы для тестового клиента

// POST запрос
func (c *TestClient) Post(url string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
}

// PUT запрос
func (c *TestClient) Put(url string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBody))

	return c.httpClient.Do(req)
}

// DELETE запрос
func (c *TestClient) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	return c.httpClient.Do(req)
}

// GET запрос с заголовками
func (c *TestClient) Get(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.httpClient.Do(req)
}

// Метод для парсинга JSON-ответа
func parseResponse(resp *http.Response, v interface{}) error {
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return err
	}

	return json.Unmarshal(body, v)
}

// Тест CRUD операций для клиентов
func TestClientCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционных тестов в коротком режиме")
	}

	it := setupTest(t)
	defer it.cleanup(t)

	clientID := fmt.Sprintf("%s_crud_%d", testClientBase, time.Now().UnixNano())

	t.Run("CreateClient", func(t *testing.T) {
		reqBody := ClientLimitRequest{
			Capacity:   10,
			RefillRate: 2,
		}

		resp, err := it.client.Post(fmt.Sprintf("%s/clients?client_id=%s", baseURL, clientID), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var response ClientLimitResponse
		err = parseResponse(resp, &response)
		require.NoError(t, err)

		assert.Equal(t, clientID, response.ClientID)
		assert.Equal(t, 10, response.Capacity)
		assert.Equal(t, 2.0, response.RefillRate)
	})

	t.Run("GetClient", func(t *testing.T) {
		// Получение информации о клиенте
		resp, err := it.client.Get(fmt.Sprintf("%s/clients/%s", baseURL, clientID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var response ClientLimitResponse
		err = parseResponse(resp, &response)
		require.NoError(t, err)

		assert.Equal(t, clientID, response.ClientID)
		assert.Equal(t, 10, response.Capacity)
		assert.Equal(t, 2.0, response.RefillRate)
	})

	t.Run("UpdateClient", func(t *testing.T) {
		// Обновление клиента
		reqBody := ClientLimitRequest{
			Capacity:   20,
			RefillRate: 5,
		}

		resp, err := it.client.Put(fmt.Sprintf("%s/clients/%s", baseURL, clientID), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Проверка обновленных данных
		resp, err = it.client.Get(fmt.Sprintf("%s/clients/%s", baseURL, clientID), nil)
		require.NoError(t, err)

		var response ClientLimitResponse
		err = parseResponse(resp, &response)
		require.NoError(t, err)

		assert.Equal(t, 20, response.Capacity)
		assert.Equal(t, 5.0, response.RefillRate)
	})

	t.Run("ListClients", func(t *testing.T) {
		// Получение списка клиентов
		resp, err := it.client.Get(fmt.Sprintf("%s/clients", baseURL), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var clients []ClientLimitResponse
		err = parseResponse(resp, &clients)
		require.NoError(t, err)

		// Проверяем, что наш клиент есть в списке
		found := false
		for _, client := range clients {
			if client.ClientID == clientID {
				found = true
				break
			}
		}
		assert.True(t, found, "Созданный клиент не найден в списке")
	})

	t.Run("DeleteClient", func(t *testing.T) {
		// Удаление клиента
		resp, err := it.client.Delete(fmt.Sprintf("%s/clients/%s", baseURL, clientID))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Проверка, что клиент действительно удален
		resp, err = it.client.Get(fmt.Sprintf("%s/clients/%s", baseURL, clientID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Проверка JSON-структуры ответа с ошибкой
		var errorResp ErrorResponse
		err = parseResponse(resp, &errorResp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, errorResp.Code)
		assert.NotEmpty(t, errorResp.Message)
	})
}

// Тест работы rate limiter
func TestRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционных тестов в коротком режиме")
	}

	it := setupTest(t)
	defer it.cleanup(t)

	clientID := fmt.Sprintf("%s_ratelimit_%d", testClientBase, time.Now().UnixNano())

	t.Run("SetupClient", func(t *testing.T) {
		// Создание клиента с настройками для rate limiting
		reqBody := ClientLimitRequest{
			Capacity:   5, // Лимит в 5 запросов
			RefillRate: 1, // 1 токен в секунду
		}

		resp, err := it.client.Post(fmt.Sprintf("%s/clients?client_id=%s", baseURL, clientID), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// Добавляем задержку для уверенности, что клиент создан
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("CheckTokenRefill", func(t *testing.T) {
		// Ждем пополнения токенов
		t.Log("Ожидание пополнения токенов...")
		time.Sleep(3 * time.Second) // Ждем, чтобы добавилось примерно 3 токена

		// Проверяем, что токены пополнились
		successCount := 0

		for i := 0; i < 3; i++ {
			req, err := http.NewRequest("GET", baseURL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-API-Key", clientID)

			resp, err := it.client.httpClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode == http.StatusOK {
				successCount++
			}
			resp.Body.Close()

			// Добавляем небольшую задержку между запросами
			time.Sleep(100 * time.Millisecond)
		}

		assert.GreaterOrEqual(t, successCount, 2, "Должно быть не менее 2 успешных запросов после пополнения")
	})
}

// Тест алгоритмов балансировки нагрузки
func TestLoadBalancing(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционных тестов в коротком режиме")
	}

	it := setupTest(t)
	defer it.cleanup(t)

	t.Run("RequestDistribution", func(t *testing.T) {
		// Отправляем несколько запросов и проверяем распределение
		responseBodies := make(map[string]int)
		requestCount := 30

		for i := 0; i < requestCount; i++ {
			// Добавляем случайный параметр, чтобы избежать кэширования
			resp, err := it.client.Get(fmt.Sprintf("%s?t=%d", baseURL, time.Now().UnixNano()), nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()

			bodyStr := string(body)
			responseBodies[bodyStr] = responseBodies[bodyStr] + 1

			// Добавляем небольшую задержку между запросами
			time.Sleep(50 * time.Millisecond)
		}

		// Проверяем, что запросы были распределены между бэкендами
		assert.GreaterOrEqual(t, len(responseBodies), 2,
			"Запросы должны быть распределены минимум между 2 бэкендами")

		// Для round-robin ожидаем равномерное распределение
		expectedPerBackend := float64(requestCount) / float64(len(responseBodies))
		tolerance := expectedPerBackend * 0.5 // 50% толерантность

		for backend, count := range responseBodies {
			diff := float64(count) - expectedPerBackend
			if diff < 0 {
				diff = -diff
			}
			assert.LessOrEqual(t, diff, tolerance,
				fmt.Sprintf("Неравномерное распределение для бэкенда %s: %d запросов", backend, count))
		}
	})
}

// Тест параллельной обработки запросов
func TestConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционных тестов в коротком режиме")
	}

	it := setupTest(t)
	defer it.cleanup(t)

	clientID := fmt.Sprintf("%s_concurrent_%d", testClientBase, time.Now().UnixNano())

	t.Run("SetupClient", func(t *testing.T) {
		// Создание клиента с достаточно большим лимитом для параллельных запросов
		reqBody := ClientLimitRequest{
			Capacity:   100,
			RefillRate: 10,
		}

		resp, err := it.client.Post(fmt.Sprintf("%s/clients?client_id=%s", baseURL, clientID), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Запуск параллельных запросов
		var wg sync.WaitGroup
		results := make(chan int, parallelCount*requestsPerGo)

		for i := 0; i < parallelCount; i++ {
			wg.Add(1)
			go func(goID int) {
				defer wg.Done()

				for j := 0; j < requestsPerGo; j++ {
					req, err := http.NewRequest("GET", fmt.Sprintf("%s?param=%d_%d", baseURL, goID, j), nil)
					if err != nil {
						t.Logf("Ошибка при создании запроса: %v", err)
						results <- -1
						continue
					}

					req.Header.Set("X-API-Key", clientID)

					resp, err := it.client.httpClient.Do(req)
					if err != nil {
						t.Logf("Ошибка при выполнении запроса: %v", err)
						results <- -1
						continue
					}

					results <- resp.StatusCode
					resp.Body.Close()
				}
			}(i)
		}

		// Ждем завершения всех горутин
		wg.Wait()
		close(results)

		// Анализируем результаты
		successCount := 0
		errorCount := 0

		for status := range results {
			if status == http.StatusOK {
				successCount++
			} else {
				errorCount++
			}
		}

		assert.Equal(t, parallelCount*requestsPerGo, successCount,
			"Все параллельные запросы должны быть успешными")
		assert.Equal(t, 0, errorCount,
			"Не должно быть ошибок при параллельных запросах")
	})
}

// Тест персистентности (сохранение в PostgreSQL)
func TestPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционных тестов в коротком режиме")
	}

	it := setupTest(t)
	defer it.cleanup(t)

	// Пропускаем тест, если нет подключения к БД
	if it.pgDB == nil {
		t.Skip("Нет подключения к PostgreSQL")
	}

	clientID := fmt.Sprintf("%s_persistence_%d", testClientBase, time.Now().UnixNano())
	capacity := 25
	refillRate := 5.0

	t.Run("SaveAndVerify", func(t *testing.T) {
		// Создание клиента
		reqBody := ClientLimitRequest{
			Capacity:   capacity,
			RefillRate: refillRate,
		}

		resp, err := it.client.Post(fmt.Sprintf("%s/clients?client_id=%s", baseURL, clientID), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// Добавляем задержку для уверенности, что данные сохранились
		time.Sleep(200 * time.Millisecond)

		// Проверяем, что данные сохранились в БД
		var dbCapacity int
		var dbRefillRate float64

		err = it.pgDB.QueryRow(
			"SELECT capacity, refill_rate FROM rate_limits WHERE client_id = $1",
			clientID,
		).Scan(&dbCapacity, &dbRefillRate)

		require.NoError(t, err, "Ошибка при получении данных из БД")
		assert.Equal(t, capacity, dbCapacity, "Capacity в БД не соответствует отправленному")
		assert.Equal(t, refillRate, dbRefillRate, "RefillRate в БД не соответствует отправленному")
	})
}

// Тест обработки ошибок
func TestErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционных тестов в коротком режиме")
	}

	it := setupTest(t)
	defer it.cleanup(t)

	t.Run("InvalidJSON", func(t *testing.T) {
		// Отправляем некорректный JSON
		req, err := http.NewRequest("POST",
			fmt.Sprintf("%s/clients?client_id=test_error", baseURL),
			bytes.NewBuffer([]byte("This is not a valid JSON")))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		resp, err := it.client.httpClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResp ErrorResponse
		err = parseResponse(resp, &errorResp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, errorResp.Code)
		assert.NotEmpty(t, errorResp.Message)
	})

	t.Run("MissingClientID", func(t *testing.T) {
		// Отправляем запрос без client_id
		reqBody := ClientLimitRequest{
			Capacity:   10,
			RefillRate: 2,
		}

		resp, err := it.client.Post(fmt.Sprintf("%s/clients", baseURL), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResp ErrorResponse
		err = parseResponse(resp, &errorResp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, errorResp.Code)
	})

	t.Run("NonexistentClient", func(t *testing.T) {
		// Запрос информации о несуществующем клиенте
		resp, err := it.client.Get(fmt.Sprintf("%s/clients/nonexistent_client", baseURL), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		var errorResp ErrorResponse
		err = parseResponse(resp, &errorResp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, errorResp.Code)
	})

	t.Run("UpdateNonexistentClient", func(t *testing.T) {
		// Обновление несуществующего клиента
		reqBody := ClientLimitRequest{
			Capacity:   10,
			RefillRate: 2,
		}

		resp, err := it.client.Put(fmt.Sprintf("%s/clients/nonexistent_update", baseURL), reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
