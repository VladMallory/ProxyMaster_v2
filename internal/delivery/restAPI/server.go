// nolint
package restapi

import (
	"fmt"
	"net/http"

	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
	"github.com/gorilla/mux"
)

// ServerAPI структура апи
type ServerAPI struct {
	logger  logger.Logger
	handler *Handler
}

// New конструктор ServerAPI
func New(Handler *Handler, logger logger.Logger) *ServerAPI {
	return &ServerAPI{
		logger:  logger,
		handler: Handler,
	}
}

// loggingMiddleware - middleware для логгирования хули
func (s *ServerAPI) loggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			s.logger.Info(fmt.Sprintf("%s %s address: %s", r.Method, r.URL.Path, r.RemoteAddr))
			handler.ServeHTTP(w, r)
		},
	)
}

// corsMiddleware - middleware для фронтенда если он будет на другом домене, без него НИ ХУ Я работать не будет
func (s *ServerAPI) corsMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*") // вместо звездочки ОБЯЗАТЕЛЬНО поставить домен фронта
			// если забуду его поставить, отхуярьте меня ногами
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS") // пока только GET и OPTIONS, если надо сами допишите

			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" { // обработка preflight запроса
				w.WriteHeader(http.StatusOK)
				return
			}

			handler.ServeHTTP(w, r)
		},
	)
}

// Serve функция которая запускает апи
func (s *ServerAPI) Serve(addr string) error {
	mux := mux.NewRouter()
	mux.Path("/url/{username}").Methods("GET").HandlerFunc(s.handler.GetSubscriptionURL)
	loggingMux := s.loggingMiddleware(mux)
	corsMux := s.corsMiddleware(loggingMux)

	if err := http.ListenAndServe(addr, corsMux); err != nil {
		s.logger.Error(
			"failed to serve rest api",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	return nil
}
