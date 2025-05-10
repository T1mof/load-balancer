package ratelimiter

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// ClientLimitRequest структура для запроса создания/обновления клиента
// Используется для парсинга JSON тела запроса
type ClientLimitRequest struct {
	Capacity   int     `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
}

// ClientLimitResponse структура для ответа с информацией о клиенте
type ClientLimitResponse struct {
	ClientID   string  `json:"client_id"`
	Capacity   int     `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
	Message    string  `json:"message,omitempty"`
}

// ErrorResponse структура для ответа с ошибкой
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CreateClientHandler обрабатывает запросы на создание нового клиента
func (rl *RateLimiter) CreateClientHandler(w http.ResponseWriter, r *http.Request) {
	var req ClientLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "client_id is required")
		return
	}

	rl.SetClientLimit(clientID, req.Capacity, req.RefillRate)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ClientLimitResponse{
		ClientID:   clientID,
		Capacity:   req.Capacity,
		RefillRate: req.RefillRate,
		Message:    "Client created successfully",
	})
}

// UpdateClientHandler обрабатывает запросы на обновление настроек клиента
func (rl *RateLimiter) UpdateClientHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["client_id"]

	var req ClientLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Проверяем существование клиента
	_, _, exists := rl.GetClientLimit(clientID)
	if !exists {
		sendErrorResponse(w, http.StatusNotFound, "Client not found")
		return
	}

	rl.SetClientLimit(clientID, req.Capacity, req.RefillRate)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ClientLimitResponse{
		ClientID:   clientID,
		Capacity:   req.Capacity,
		RefillRate: req.RefillRate,
		Message:    "Client updated successfully",
	})
}

// GetClientHandler обрабатывает запросы на получение информации о клиенте
func (rl *RateLimiter) GetClientHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["client_id"]

	capacity, refillRate, exists := rl.GetClientLimit(clientID)
	if !exists {
		sendErrorResponse(w, http.StatusNotFound, "Client not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ClientLimitResponse{
		ClientID:   clientID,
		Capacity:   capacity,
		RefillRate: refillRate,
	})
}

// DeleteClientHandler обрабатывает запросы на удаление клиента
func (rl *RateLimiter) DeleteClientHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["client_id"]

	// Проверяем существование клиента
	_, _, exists := rl.GetClientLimit(clientID)
	if !exists {
		sendErrorResponse(w, http.StatusNotFound, "Client not found")
		return
	}

	err := rl.DeleteClientLimit(clientID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to delete client")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Client deleted successfully",
	})
}

// ListClientsHandler обрабатывает запросы на получение списка всех клиентов
func (rl *RateLimiter) ListClientsHandler(w http.ResponseWriter, r *http.Request) {
	clients := rl.GetAllClients()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// RegisterClientRoutes регистрирует все маршруты API для управления клиентами
func (rl *RateLimiter) RegisterClientRoutes(router *mux.Router) {
	router.HandleFunc("/clients", rl.ListClientsHandler).Methods("GET")
	router.HandleFunc("/clients", rl.CreateClientHandler).Methods("POST")
	router.HandleFunc("/clients/{client_id}", rl.GetClientHandler).Methods("GET")
	router.HandleFunc("/clients/{client_id}", rl.UpdateClientHandler).Methods("PUT")
	router.HandleFunc("/clients/{client_id}", rl.DeleteClientHandler).Methods("DELETE")
}

// sendErrorResponse отправляет структурированный JSON-ответ с ошибкой
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Code:    statusCode,
		Message: message,
	})
}
