package ctftime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGetLogoRejectsBadResponses(t *testing.T) {
	tests := map[string]*httptest.Server{
		"error status":             serve(t, http.StatusNotFound, "not found page, long enough"),
		"body shorter than a logo": serve(t, http.StatusOK, "x"),
		"empty body":               serve(t, http.StatusOK, ""),
	}
	for name, srv := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := GetLogo(srv.URL); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestGetCTFsReportsErrorStatus(t *testing.T) {
	old := BaseURL
	t.Cleanup(func() { BaseURL = old })
	BaseURL = serve(t, http.StatusBadGateway, "[]").URL

	if _, err := GetCTFs(5, true); err == nil {
		t.Fatal("an API failure must not look like an empty list")
	}
}

func TestHTTPClientHasTimeout(t *testing.T) {
	if httpClient.Timeout <= 0 {
		t.Fatal("a request without timeout can freeze the bot")
	}
}
