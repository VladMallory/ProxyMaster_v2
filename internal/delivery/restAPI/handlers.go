// Package restapi содержит хэндлеры для REST API.
// nolint
package restapi

import (
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/models"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Handler структура отвечает за хэндлеры.
type Handler struct {
	remnawaveClient     *remnawave.RemnaClient
	subscriptionService domain.SubscriptionService
}

// NewHandler конструктор Handler
func NewHandler(
	remnawaveClient *remnawave.RemnaClient,
	subscriptionService domain.SubscriptionService,
) *Handler {
	return &Handler{
		remnawaveClient:     remnawaveClient,
		subscriptionService: subscriptionService,
	}
}

// respondWithJSON - вспомагательная функция для отправки JSON ответа
func (h *Handler) responseWithJSON(w http.ResponseWriter, payload any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		return
	}
}

// respondError - тоже самое что и respondWithJSON, но название другое для удобства
// ну и почему бы нет
func (h *Handler) respondError(w http.ResponseWriter, payload any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		return
	}
}

// newPayload конструктор тела ответа
func (h *Handler) newPayload(statusCode int, message string) *models.ErrResponseRestAPI {
	return &models.ErrResponseRestAPI{
		StatusCode: statusCode,
		ErrMessage: message,
	}
}

// GetSubscriptionURL хэндлер роута /url/{username}, возвращает SubscriptionURL
// по имени пользователя
func (h *Handler) GetSubscriptionURL(w http.ResponseWriter, r *http.Request) {
	username := mux.Vars(r)["username"]

	if username == "" {
		h.respondError(w, "username cannot be empty", http.StatusBadRequest)
		return
	}

	url, err := h.subscriptionService.GetURLSubscription(username)

	switch err {
	case remnawave.ErrBadRequestUsername:
		h.respondError(w, h.newPayload(http.StatusBadRequest, "bad request"), http.StatusBadRequest)
		return
	case remnawave.ErrInternalServerError:
		h.respondError(
			w,
			h.newPayload(http.StatusInternalServerError, "internal server error"),
			http.StatusInternalServerError,
		)
		return
	case remnawave.ErrNotFound:
		h.respondError(w, h.newPayload(http.StatusNotFound, "user not found"), http.StatusNotFound)
		return
	}

	if url == "" {
		h.respondError(w, "failed to get url", http.StatusInternalServerError)
		return
	}

	payload := models.GetSubscriptionURLResponse{URL: url}
	h.responseWithJSON(w, payload, http.StatusOK)
}
