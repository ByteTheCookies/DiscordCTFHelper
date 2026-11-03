package ctftime

import (
	"errors"
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

func TestLogoRejectsBadResponses(t *testing.T) {
	tests := map[string]*httptest.Server{
		"error status":             serve(t, http.StatusNotFound, "not found page, long enough"),
		"body shorter than a logo": serve(t, http.StatusOK, "x"),
		"empty body":               serve(t, http.StatusOK, ""),
	}
	for name, srv := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := New("").Logo(t.Context(), srv.URL); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestUpcomingReportsErrorStatus(t *testing.T) {
	c := New(serve(t, http.StatusBadGateway, "[]").URL)
	if _, err := c.Upcoming(t.Context(), 5, true); err == nil {
		t.Fatal("an API failure must not look like an empty list")
	}
}

func TestUpcomingFiltersOnsite(t *testing.T) {
	c := New(serve(t, http.StatusOK, `[{"id":1,"onsite":true},{"id":2,"onsite":false},{"id":3,"onsite":true}]`).URL)

	events, err := c.Upcoming(t.Context(), 5, false)
	if err != nil || len(events) != 1 || events[0].ID != 2 {
		t.Fatalf("online only = %+v, err %v", events, err)
	}
	events, err = c.Upcoming(t.Context(), 5, true)
	if err != nil || len(events) != 3 {
		t.Fatalf("all = %+v, err %v", events, err)
	}
}

func TestResults(t *testing.T) {
	c := New(serve(t, http.StatusOK, `{"7":{"scores":[{"team_id":1,"place":9,"points":"1"},{"team_id":42,"place":2,"points":" 123.5 "}]}}`).URL)

	got, err := c.Results(t.Context(), 7, 2026, 42)
	if err != nil || got != (Result{Place: 2, Score: 123}) {
		t.Fatalf("result = %+v, err %v", got, err)
	}
	if _, err := c.Results(t.Context(), 7, 2026, 5); !errors.Is(err, ErrNoResults) {
		t.Fatalf("unknown team err = %v", err)
	}
	if _, err := c.Results(t.Context(), 8, 2026, 42); !errors.Is(err, ErrNoResults) {
		t.Fatalf("unknown event err = %v", err)
	}
}

func TestClientHasTimeout(t *testing.T) {
	if New("").http.Timeout <= 0 {
		t.Fatal("a request without timeout can hang forever")
	}
}
