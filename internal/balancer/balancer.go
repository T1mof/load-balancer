package balancer

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"load-balancer/internal/logger"
)

// Server представляет бэкенд-сервер
type Server struct {
	URL               *url.URL
	ReverseProxy      *httputil.ReverseProxy
	ActiveConnections int
	Healthy           bool
	mutex             sync.RWMutex
}

// LoadBalancer содержит пул серверов и стратегию распределения
type LoadBalancer struct {
	servers   []*Server
	algorithm BalancingAlgorithm
	logger    *logger.Logger
	mutex     sync.RWMutex
}

// BalancingAlgorithm определяет стратегию выбора сервера
type BalancingAlgorithm interface {
	NextServer(servers []*Server) *Server
}

// getNextServer возвращает следующий доступный сервер
func (lb *LoadBalancer) getNextServer() *Server {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()
	return lb.algorithm.NextServer(lb.servers)
}

// ServeHTTP обрабатывает HTTP-запросы
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Выбираем сервер используя текущий алгоритм
	server := lb.getNextServer()

	if server == nil {
		http.Error(w, "Все серверы недоступны", http.StatusServiceUnavailable)
		return
	}

	// Увеличиваем счетчик активных соединений
	server.mutex.Lock()
	server.ActiveConnections++
	server.mutex.Unlock()

	// Логируем запрос
	lb.logger.Infof("Запрос %s перенаправлен на %s", r.URL.Path, server.URL.Host)

	// Перенаправляем запрос на выбранный сервер
	server.ReverseProxy.ServeHTTP(w, r)

	// Уменьшаем счетчик активных соединений
	server.mutex.Lock()
	server.ActiveConnections--
	server.mutex.Unlock()
}

// IsHealthy проверяет, доступен ли сервер
func (s *Server) IsHealthy() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Healthy
}

// SetHealth устанавливает статус здоровья сервера
func (s *Server) SetHealth(healthy bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Healthy = healthy
}

// NewLoadBalancer создает новый балансировщик нагрузки
func NewLoadBalancer(backends []string, algorithmName string, logger *logger.Logger) (*LoadBalancer, error) {
	servers := make([]*Server, 0, len(backends))

	for _, backend := range backends {
		url, err := url.Parse(backend)
		if err != nil {
			return nil, fmt.Errorf("неверный формат URL %s: %v", backend, err)
		}

		proxy := httputil.NewSingleHostReverseProxy(url)

		// Настройка обработки ошибок при проксировании
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Errorf("Ошибка проксирования запроса к %s: %v", backend, err)
			http.Error(w, "Ошибка при проксировании запроса", http.StatusBadGateway)
		}

		server := &Server{
			URL:               url,
			ReverseProxy:      proxy,
			ActiveConnections: 0,
			Healthy:           true,
		}

		servers = append(servers, server)
	}

	// Выбираем алгоритм балансировки
	var algorithm BalancingAlgorithm
	switch algorithmName {
	case "round-robin":
		algorithm = NewRoundRobin()
	case "least-connections":
		algorithm = NewLeastConnections()
	default:
		return nil, fmt.Errorf("неизвестный алгоритм балансировки: %s", algorithmName)
	}

	return &LoadBalancer{
		servers:   servers,
		algorithm: algorithm,
		logger:    logger,
	}, nil
}

// Servers возвращает срез серверов балансировщика
func (lb *LoadBalancer) Servers() []*Server {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()
	return lb.servers
}
