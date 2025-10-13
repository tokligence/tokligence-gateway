package client

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
)

type stubHTTPClient struct {
	handler func(*http.Request) (*http.Response, error)
}

func (s *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return s.handler(req)
}

func TestRegisterAndListProviders(t *testing.T) {
	calls := 0
	stub := &stubHTTPClient{
		handler: func(req *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				if req.Method != http.MethodPost || req.URL.Path != "/users" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				body := io.NopCloser(strings.NewReader(`{"user":{"id":1,"email":"agent@example.com","roles":["consumer"]}}`))
				return &http.Response{StatusCode: http.StatusCreated, Body: body, Header: make(http.Header)}, nil
			case 2:
				if req.Method != http.MethodGet || req.URL.Path != "/providers" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				body := io.NopCloser(strings.NewReader(`{"providers":[{"id":10,"user_id":1,"display_name":"Team","description":""}]}`))
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
			default:
				t.Fatalf("unexpected call %d", calls)
				return nil, nil
			}
		},
	}

	client, err := NewMarketplaceClient("http://example.com", stub)
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}
	client.SetLogger(log.New(io.Discard, "", 0))

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
