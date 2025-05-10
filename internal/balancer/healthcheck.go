package balancer

import (
	"fmt"
	"net/http"
	"time"

	"load-balancer/internal/logger"
)

// HealthChecker выполняет проверку доступности серверов
type HealthChecker struct {
	servers        []*Server
	checkInterval  time.Duration
	healthEndpoint string
	client         *http.Client
	logger         *logger.Logger
	stopChan       chan struct{}
}

// NewHealthChecker создает новый checker для проверки здоровья серверов
func NewHealthChecker(servers []*Server, interval time.Duration, endpoint string, logger *logger.Logger) *HealthChecker {
	return &HealthChecker{
		servers:        servers,
		checkInterval:  interval,
		healthEndpoint: endpoint,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Start запускает периодическую проверку серверов
func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.checkInterval)

	go func() {
		hc.checkServers()

		for {
			select {
			case <-ticker.C:
				hc.checkServers()
			case <-hc.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop останавливает проверки здоровья
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
}

// checkServers проверяет доступность всех серверов
func (hc *HealthChecker) checkServers() {
	for _, server := range hc.servers {
		go hc.checkServer(server)
	}
}

// checkServer проверяет доступность отдельного сервера
func (hc *HealthChecker) checkServer(server *Server) {
	healthURL := fmt.Sprintf("http://%s%s", server.URL.Host, hc.healthEndpoint)

	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		hc.logger.Errorf("Ошибка создания запроса для %s: %v", server.URL.Host, err)
		return
	}

	resp, err := hc.client.Do(req)

	wasHealthy := server.IsHealthy()

	if err != nil {
		server.SetHealth(false)
		if wasHealthy {
			hc.logger.Warnf("Сервер %s помечен как недоступный: %v", server.URL.Host, err)
		}
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		server.SetHealth(false)
		if wasHealthy {
			hc.logger.Warnf("Сервер %s помечен как недоступный: код ответа %d", server.URL.Host, resp.StatusCode)
		}
		return
	}

	server.SetHealth(true)
	if !wasHealthy {
		hc.logger.Infof("Сервер %s снова доступен", server.URL.Host)
	}
}
