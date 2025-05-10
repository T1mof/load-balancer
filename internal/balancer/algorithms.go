package balancer

import (
	"sync"
)

// RoundRobin реализует алгоритм Round Robin
type RoundRobin struct {
	current int
	mutex   sync.Mutex
}

// NewRoundRobin создает новый RoundRobin
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		current: -1,
	}
}

// NextServer выбирает следующий сервер по Round Robin
func (rr *RoundRobin) NextServer(servers []*Server) *Server {
	if len(servers) == 0 {
		return nil
	}

	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	initialIdx := rr.current

	for i := 0; i < len(servers); i++ {
		rr.current = (rr.current + 1) % len(servers)
		server := servers[rr.current]

		if server.IsHealthy() {
			return server
		}

		if rr.current == initialIdx {
			break
		}
	}

	// Если не нашли здоровый сервер, возвращаем nil
	return nil
}

// LeastConnections реализует алгоритм выбора сервера с наименьшим количеством соединений
type LeastConnections struct{}

// NewLeastConnections создает новый LeastConnections
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

// NextServer выбирает сервер с наименьшим количеством активных соединений
func (lc *LeastConnections) NextServer(servers []*Server) *Server {
	if len(servers) == 0 {
		return nil
	}

	var minServer *Server
	minConnections := -1

	for _, server := range servers {
		if !server.IsHealthy() {
			continue
		}

		connections := server.ActiveConnections

		if minServer == nil || connections < minConnections {
			minConnections = connections
			minServer = server
		}
	}

	return minServer
}
