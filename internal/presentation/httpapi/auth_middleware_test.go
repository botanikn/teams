package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tokenIssuerStub struct{}

func (tokenIssuerStub) Issue(int64) (string, error) { return "", nil }
func (tokenIssuerStub) Parse(token string) (int64, error) {
	if token != "valid-token" {
		return 0, errors.New("invalid token")
	}
	return 42, nil
}

func TestAuthenticateRequiresValidBearerToken(t *testing.T) {
	handler := authenticate(tokenIssuerStub{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUserID(r) != 42 {
			t.Errorf("currentUserID() = %d, want 42", currentUserID(r))
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "invalid scheme", header: "Basic valid-token", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer invalid-token", wantStatus: http.StatusUnauthorized},
		{name: "valid token", header: "Bearer valid-token", wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Code == http.StatusUnauthorized && !strings.Contains(response.Body.String(), "authentication required") {
				t.Fatalf("unexpected authentication response: %s", response.Body.String())
			}
		})
	}
}
