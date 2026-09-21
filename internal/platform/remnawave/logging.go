package platformremnawave

import (
	"net/http"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/platform/traceid"
	"go.uber.org/zap"
)

func withLogging(logger *zap.Logger) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			reqID := traceid.From(r.Context())
			_ = reqID
			username := traceid.UsernameFrom(r.Context())
			start := time.Now()

			resp, err := next.RoundTrip(r)

			duration := time.Since(start)

			fields := []zap.Field{
				// zap.String("req_id", reqID),
				// zap.String("method", r.Method),
				zap.String("username", username),
				zap.String("path", r.URL.Path),
				zap.Duration("duration_ms", duration),
			}

			if err != nil {
				logger.Error(
					"remnawave request failed",
					append(fields, zap.Error(err))...,
				)

				return nil, err
			}

			// fields = append(fields, zap.Int("status", resp.StatusCode))
			switch {
			case resp.StatusCode >= 500:
				logger.Error(
					"remnawave request failed",
					fields...,
				)
			case duration > 1*time.Second:
				logger.Warn(
					"remnawave request slow",
					fields...,
				)
			default:
				logger.Info(
					"remnawave request done",
					fields...,
				)
			}

			return resp, nil
		})
	}
}
