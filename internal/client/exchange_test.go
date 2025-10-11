package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterAndListProviders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"user":{"id":1,"email":"agent@example.com","roles":["consumer"]}}`))
	})
	mux.HandleFunc("/providers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"providers":[{"id":10,"user_id":1,"display_name":"Team","description":""}]}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := NewExchangeClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewExchangeClient: %v", err)
	}

	_, err = client.RegisterUser(context.Background(), RegisterUserRequest{Email: "agent@example.com", Roles: []string{"consumer"}})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	providers, err := client.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}

	if len(providers) != 1 || providers[0].ID != 10 {
		t.Fatalf("unexpected providers: %#v", providers)
	}
}
