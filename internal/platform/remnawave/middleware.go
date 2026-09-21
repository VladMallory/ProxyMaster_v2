package platformremnawave

import (
	"net/http"
	"slices"
)

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type Middleware func(next http.RoundTripper) http.RoundTripper

func chain(base http.RoundTripper, mws ...Middleware) http.RoundTripper {
	for _, mw := range slices.Backward(mws) {
		base = mw(base)
	}

	return base
}
