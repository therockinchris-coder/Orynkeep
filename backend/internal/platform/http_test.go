package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadiness(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure error
		status  int
	}{
		{"available", nil, http.StatusOK},
		{"unavailable", errors.New("secret database connection detail"), http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := Handler(func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("missing readiness deadline")
				}
				return test.failure
			})
			response := httptest.NewRecorder()
			h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health/ready", nil))
			if response.Code != test.status {
				t.Fatalf("status = %d", response.Code)
			}
			if strings.Contains(response.Body.String(), "secret") {
				t.Fatal("readiness leaked internal details")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health response may be cached")
			}
		})
	}
}

func TestLivenessDoesNotDependOnDatabase(t *testing.T) {
	h := Handler(func(context.Context) error { t.Fatal("liveness checked database"); return nil })
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}
