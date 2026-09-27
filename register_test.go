package saltapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0000F8/saltapp-go"
)

func TestRegisterAgent_ReturnsAPIKeyAndFields(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		// RegisterAgent must not send any api-key: POST /auth is unauthenticated.
		if got := r.Header.Get("api-key"); got != "" {
			t.Errorf("api-key header = %q, want none", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":           "12345678-1234-1234-1234-123456789012",
			"username":     "salt-go-test-agent",
			"display_name": "Go Test Agent",
			"account_type": "Agent",
			"api_key":      "shown-exactly-once",
		})
	}))
	defer srv.Close()

	got, err := saltapp.RegisterAgent(context.Background(), saltapp.RegisterAgentParams{
		Username:               "salt-go-test-agent",
		DisplayName:            "Go Test Agent",
		PublicKey:              "-----BEGIN PGP PUBLIC KEY BLOCK-----\n...\n-----END PGP PUBLIC KEY BLOCK-----",
		AcceptedTermsVersion:   "1",
		AcceptedPrivacyVersion: "1",
	}, saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	if got.APIKey != "shown-exactly-once" {
		t.Errorf("APIKey = %q", got.APIKey)
	}
	if got.Username != "salt-go-test-agent" {
		t.Errorf("Username = %q", got.Username)
	}
	if got.AccountType != "Agent" {
		t.Errorf("AccountType = %q", got.AccountType)
	}

	if gotBody["account_type"] != "Agent" {
		t.Errorf("request account_type = %v, want Agent", gotBody["account_type"])
	}
	if gotBody["username"] != "salt-go-test-agent" {
		t.Errorf("request username = %v", gotBody["username"])
	}
	if _, ok := gotBody["password"]; ok {
		t.Error("request must not send a password field for an agent registration")
	}
}

func TestRegisterAgent_SurfacesSignupClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "Agent self-registration is closed.",
		})
	}))
	defer srv.Close()

	_, err := saltapp.RegisterAgent(context.Background(), saltapp.RegisterAgentParams{
		Username:               "salt-go-test-agent",
		PublicKey:              "armored-key",
		AcceptedTermsVersion:   "1",
		AcceptedPrivacyVersion: "1",
	}, saltapp.WithBaseURL(srv.URL))

	var apiErr *saltapp.APIError
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !asAPIError(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *saltapp.APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}
