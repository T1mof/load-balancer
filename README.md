HTTP Load Balancer
HTTP Load Balancer - высокопроизводительный балансировщик нагрузки для HTTP-запросов, написанный на Go. Он распределяет входящий трафик между несколькими бэкенд-серверами, обеспечивает отказоустойчивость и включает механизм ограничения частоты запросов (rate limiting).

Содержание
Возможности

Архитектура

Требования

Установка и запуск

Конфигурация

API для управления клиентами

Тестирование

Примеры использования

Возможности
Балансировка нагрузки: равномерное распределение HTTP-запросов между несколькими бэкенд-серверами

Несколько алгоритмов балансировки:

Round Robin - последовательное перенаправление запросов

Least Connections - перенаправление на сервер с наименьшим количеством активных соединений

Проверка доступности серверов: автоматическое определение недоступных серверов и исключение их из обработки

Rate Limiting на основе Token Bucket алгоритма:

Индивидуальные настройки для разных клиентов

Идентификация клиентов по IP-адресу или API-ключу

Хранение настроек:

In-memory хранилище

PostgreSQL для долговременного хранения

Управление клиентами через REST API

Graceful Shutdown: корректное завершение работы

Docker-интеграция: полная поддержка контейнеризации

Архитектура проекта
text
/load-balancer
├── cmd/
│   └── loadbalancer/     # Точка входа приложения
├── internal/
│   ├── balancer/         # Реализация балансировки
│   ├── config/           # Работа с конфигурацией
│   └── logger/           # Логирование
├── pkg/
│   ├── ratelimiter/      # Ограничение частоты запросов
│   └── storage/          # Хранение настроек (memory/postgres)
├── tests/                # Интеграционные тесты
├── backend/              # Тестовые бэкенд-серверы
├── config.yaml           # Пример конфигурации
├── Dockerfile            # Для сборки основного сервиса
├── Dockerfile.test       # Для запуска тестов
└── docker-compose.yml    # Запуск всех компонентов
Требования
Go 1.19 или выше

PostgreSQL (опционально, для персистентного хранения)

Docker и Docker Compose (для запуска в контейнерах)

Установка и запуск
Локальный запуск
Клонирование репозитория:

bash
git clone https://github.com/username/load-balancer.git
cd load-balancer
Установка зависимостей:

bash
go mod download
Сборка проекта:

bash
go build -o loadbalancer ./cmd/loadbalancer
Запуск:

bash
./loadbalancer --config=config.yaml
Использование Docker
Сборка и запуск с Docker Compose:

bash
docker-compose up -d
Проверка статуса контейнеров:

bash
docker-compose ps
Просмотр логов:

bash
docker-compose logs -f loadbalancer
Остановка:

bash
docker-compose down
Конфигурация
Настройки выполняются через YAML-файл:

text
server:
  port: "8080"

backends:
  - "http://backend1:80"
  - "http://backend2:80"
  - "http://backend3:80"

healthcheck:
  endpoint: "/health"
  interval: 5s

balancer:
  algorithm: "round-robin"  # или "least-connections"

ratelimit:
  default:
    capacity: 100
    refill_rate: 10  # токенов в секунду

storage:
  type: "postgres"  # или "memory"
  postgres:
    host: "postgres"
    port: 5432
    user: "postgres"
    password: "secret"
    dbname: "loadbalancer"
    sslmode: "disable"
API для управления клиентами
Получение списка всех клиентов
text
GET /clients
Пример ответа:

json
[
  {
    "client_id": "api:my-api-key",
    "capacity": 100,
    "refill_rate": 10
  },
  {
    "client_id": "ip:192.168.1.1",
    "capacity": 50,
    "refill_rate": 5
  }
]
Создание нового клиента
text
POST /clients?client_id=user123
Тело запроса:

json
{
  "capacity": 200,
  "refill_rate": 20
}
Пример ответа:

json
{
  "client_id": "user123",
  "capacity": 200,
  "refill_rate": 20,
  "message": "Client created successfully"
}
Получение информации о клиенте
text
GET /clients/{client_id}
Пример ответа:

json
{
  "client_id": "user123",
  "capacity": 200,
  "refill_rate": 20
}
Обновление настроек клиента
text
PUT /clients/{client_id}
Тело запроса:

json
{
  "capacity": 300,
  "refill_rate": 30
}
Пример ответа:

json
{
  "client_id": "user123",
  "capacity": 300,
  "refill_rate": 30,
  "message": "Client updated successfully"
}
Удаление клиента
text
DELETE /clients/{client_id}
Пример ответа:

json
{
  "message": "Client deleted successfully"
}
Тестирование
Запуск интеграционных тестов
bash
# С использованием Docker
docker-compose run tests

# Локально
go test -v ./tests
Нагрузочное тестирование
Можно использовать инструменты вроде Apache Bench (ab) или wrk:

bash
# Отправка 10000 запросов с 100 параллельными соединениями
ab -n 10000 -c 100 http://localhost:8080/
Примеры использования
Балансировка запросов
Все запросы к балансировщику автоматически распределяются между бэкендами:

bash
curl http://localhost:8080/your-endpoint
Использование API-ключа для авторизации и ограничения
bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/your-endpoint
Создание клиента с индивидуальными настройками
bash
curl -X POST -d '{"capacity": 500, "refill_rate": 50}' \
  -H "Content-Type: application/json" \
  http://localhost:8080/clients?client_id=premium_user
Производительность
Пропускная способность: до 5000 запросов в секунду на одном экземпляре

Низкая задержка: P95 < 20ms

Эффективное потребление ресурсов: ~50MB RAM в базовой конфигурации