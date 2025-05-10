package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"load-balancer/internal/balancer"
	"load-balancer/internal/config"
	"load-balancer/internal/logger"
	"load-balancer/pkg/ratelimiter"
	"load-balancer/pkg/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Инициализация логгера
	log := logger.NewLogger()
	log.Info("Запуск балансировщика нагрузки")

	// Загрузка конфигурации
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Инициализация хранилища
	var store storage.Storage
	if cfg.Storage.Type == "postgres" {
		pgConfig := storage.Config{
			Host:     cfg.Storage.Postgres.Host,
			Port:     cfg.Storage.Postgres.Port,
			User:     cfg.Storage.Postgres.User,
			Password: cfg.Storage.Postgres.Password,
			DBName:   cfg.Storage.Postgres.DBName,
			SSLMode:  cfg.Storage.Postgres.SSLMode,
		}

		pgStorage, err := storage.NewPostgresStorage(pgConfig)
		if err != nil {
			log.Fatalf("Ошибка инициализации PostgreSQL: %v", err)
		}
		defer pgStorage.Close()
		store = pgStorage

		log.Info("Подключено к PostgreSQL")
	} else {
		log.Info("Используется хранилище в памяти")
		store = storage.NewMemoryStorage()
	}

	// Создание балансировщика
	lb, err := balancer.NewLoadBalancer(cfg.Backends, cfg.Balancer.Algorithm, log)
	if err != nil {
		log.Fatalf("Ошибка создания балансировщика: %v", err)
	}

	// Настройка проверки здоровья
	hc := balancer.NewHealthChecker(
		lb.Servers(),
		cfg.HealthCheck.Interval,
		cfg.HealthCheck.Endpoint,
		log,
	)
	hc.Start()

	// Создание rate limiter
	limiter := ratelimiter.NewRateLimiter(
		cfg.RateLimit.Default.Capacity,
		cfg.RateLimit.Default.RefillRate,
		log,
		store,
	)

	// Создаем маршрутизатор для API
	router := mux.NewRouter()

	// Регистрируем маршруты для управления клиентами
	limiter.RegisterClientRoutes(router)

	// Создаем мультиплексор для обработки разных типов запросов
	mainMux := http.NewServeMux()

	// Запросы к API обрабатываются через router
	mainMux.Handle("/clients", router)
	mainMux.Handle("/clients/", router)

	// Все остальные запросы проходят через rate limiter и направляются на балансировщик
	mainMux.Handle("/", ratelimiter.RateLimitMiddleware(limiter)(lb))

	// Создание HTTP-сервера с новым обработчиком
	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: mainMux,
	}

	go func() {
		log.Infof("Сервер запущен на порту %s", cfg.Server.Port)
		log.Info("API для управления клиентами доступен по адресу: http://localhost:" + cfg.Server.Port + "/clients")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Завершение работы сервера...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при завершении работы сервера: %v", err)
	}

	log.Info("Сервер остановлен")
}
