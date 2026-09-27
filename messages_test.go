package saltapp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0000F8/saltapp-go"
)

func TestSendPlainMessage_SendsEncryptedFalseAndReturnsMessageID(t *testing.T) {
	const (
		chatID    = "11111111-2222-3333-4444-555555555555"
		messageID = "66666666-7777-8888-9999-000000000000"
	)

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/messages" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"chat_id":    chatID,
			"message_id": messageID,
			"seq":        1,
			"message":    "hello, room",
			"encrypted":  false,
			"user": map[string]any{
				"id":           "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"username":     "salt-go-test-agent",
				"display_name": "Go Test Agent",
				"account_type": "Agent",
			},
		})
	}))
	defer srv.Close()

	client, err := saltapp.New("test-key", saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	gotID, err := client.SendPlainMessage(context.Background(), chatID, "hello, room")
	if err != nil {
		t.Fatalf("SendPlainMessage: %v", err)
	}
	if gotID != messageID {
		t.Errorf("messageID = %q, want %q", gotID, messageID)
	}
	if gotBody["chat_id"] != chatID {
		t.Errorf("request chat_id = %v, want %v", gotBody["chat_id"], chatID)
	}
	if gotBody["message"] != "hello, room" {
		t.Errorf("request message = %v", gotBody["message"])
	}
	encrypted, ok := gotBody["encrypted"].(bool)
	if !ok || encrypted != false {
		t.Errorf("request encrypted = %#v, want explicit false", gotBody["encrypted"])
	}
}

// TestSendPlainMessage_SurfacesEncryptedRoomRefusal proves the API's refusal
// to accept plaintext against an encrypted chat (422, per the OpenAPI
// document's description of createMessage) comes back as a typed *APIError
// rather than being swallowed.
func TestSendPlainMessage_SurfacesEncryptedRoomRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "This room is encrypted. Messages must be sent encrypted.",
		})
	}))
	defer srv.Close()

	client, err := saltapp.New("test-key", saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.SendPlainMessage(context.Background(), "some-chat-id", "hi")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *saltapp.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *saltapp.APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if len(apiErr.Messages) != 1 || apiErr.Messages[0] != "This room is encrypted. Messages must be sent encrypted." {
		t.Errorf("Messages = %#v", apiErr.Messages)
	}
}
