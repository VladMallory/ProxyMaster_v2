package http

import "net/http"

// RegisterRoutes регистрирует HTTP роуты подписки на переданный mux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("GET /api/v1/subscription/{userID}/url", h.GetSubscriptionURL)
	mux.HandleFunc("POST /api/v1/subscription/activate", h.ActivateSubscription)
}
