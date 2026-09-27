package saltapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0000F8/saltapp-go"
)

// messageFixture builds a response body matching the OpenAPI document's
// Message schema (components.schemas.Message): required chat_id, message_id,
// seq, user, plus resource_id/resource_type for a just-created card. Message
// has NO top-level "id" -- the document names these resource_id/message_id,
// which is exactly what PostCard must read.
func messageFixture(t *testing.T, chatID, messageID, resourceID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"chat_id":       chatID,
		"message_id":    messageID,
		"seq":           1,
		"message_type":  "card",
		"resource_type": "Card",
		"resource_id":   resourceID,
		"user": map[string]any{
			"id":           "11111111-1111-1111-1111-111111111111",
			"username":     "salt-go-test-agent",
			"display_name": "Go Test Agent",
			"account_type": "Agent",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// cardResourceFixture builds a response body matching the OpenAPI document's
// CardResource schema, with `interactions` (owner-only) populated per the
// CardResource.interactions item schema: id, user_id, action_id, value,
// created_at, transfer_request_id, transfer_request_status.
func cardResourceFixture(cardID string, interactions []map[string]any) []byte {
	body, _ := json.Marshal(map[string]any{
		"id":        cardID,
		"card_type": "blocks",
		"owner": map[string]any{
			"id":           "22222222-2222-2222-2222-222222222222",
			"username":     "salt-go-test-owner",
			"display_name": "Go Test Owner",
			"account_type": "Agent",
		},
		"interactions": interactions,
	})
	return body
}

func TestPostCard_ExtractsResourceIDAndMessageID(t *testing.T) {
	const (
		chatID     = "33333333-3333-3333-3333-333333333333"
		messageID  = "44444444-4444-4444-4444-444444444444"
		resourceID = "55555555-5555-5555-5555-555555555555"
	)

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cards" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Fatalf("api-key header = %q, want %q", got, "test-key")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(messageFixture(t, chatID, messageID, resourceID))
	}))
	defer srv.Close()

	client, err := saltapp.New("test-key", saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	gotCardID, gotMessageID, err := client.PostCard(context.Background(), chatID, "pick one", []saltapp.CardBlock{
		saltapp.SectionBlock("Pick one"),
		saltapp.ActionsBlock(
			saltapp.Button("yes", "Yes"),
			saltapp.PayButton("pay", "Pay $1", "1.00", "USDC"),
		),
	})
	if err != nil {
		t.Fatalf("PostCard: %v", err)
	}
	if gotCardID != resourceID {
		t.Errorf("cardID = %q, want resource_id %q", gotCardID, resourceID)
	}
	if gotMessageID != messageID {
		t.Errorf("messageID = %q, want message_id %q", gotMessageID, messageID)
	}

	if gotBody["chat_id"] != chatID {
		t.Errorf("request chat_id = %v, want %v", gotBody["chat_id"], chatID)
	}
	blocks, ok := gotBody["blocks"].([]any)
	if !ok || len(blocks) != 2 {
		t.Fatalf("request blocks = %#v, want 2 blocks", gotBody["blocks"])
	}
	actions, ok := blocks[1].(map[string]any)
	if !ok || actions["type"] != "actions" {
		t.Fatalf("second block = %#v, want an actions block", blocks[1])
	}
	elements, ok := actions["elements"].([]any)
	if !ok || len(elements) != 2 {
		t.Fatalf("actions elements = %#v, want 2 buttons", actions["elements"])
	}
	payButton, ok := elements[1].(map[string]any)
	if !ok || payButton["action_type"] != "pay" {
		t.Fatalf("second button = %#v, want action_type pay", elements[1])
	}
	pay, ok := payButton["pay"].(map[string]any)
	if !ok || pay["amount"] != "1.00" || pay["currency"] != "USDC" {
		t.Fatalf("pay button pay = %#v", payButton["pay"])
	}
}

func TestGetCard_ParsesInteractions(t *testing.T) {
	const cardID = "66666666-6666-6666-6666-666666666666"

	var gotAfter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAfter = r.URL.Query().Get("after")
		w.Header().Set("Content-Type", "application/json")
		w.Write(cardResourceFixture(cardID, []map[string]any{
			{
				"id":                      "77777777-7777-7777-7777-777777777777",
				"user_id":                 "88888888-8888-8888-8888-888888888888",
				"action_id":               "yes",
				"value":                   nil,
				"created_at":              "2026-09-27T00:00:00Z",
				"transfer_request_id":     "99999999-9999-9999-9999-999999999999",
				"transfer_request_status": "confirmed",
			},
		}))
	}))
	defer srv.Close()

	client, err := saltapp.New("test-key", saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	card, err := client.GetCard(context.Background(), cardID, "cursor-1")
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if gotAfter != "cursor-1" {
		t.Errorf("after query = %q, want %q", gotAfter, "cursor-1")
	}
	if card.ID != cardID {
		t.Errorf("card.ID = %q, want %q", card.ID, cardID)
	}
	if len(card.Interactions) != 1 {
		t.Fatalf("card.Interactions = %#v, want 1 entry", card.Interactions)
	}
	got := card.Interactions[0]
	if got.ActionID != "yes" {
		t.Errorf("ActionID = %q, want %q", got.ActionID, "yes")
	}
	if got.TransferRequestStatus != "confirmed" {
		t.Errorf("TransferRequestStatus = %q, want %q", got.TransferRequestStatus, "confirmed")
	}
}

// TestWaitForTap_ThreadsAfterAndPaces proves two things at once: the same
// `after` cursor given to WaitForTap is threaded into every GetCard poll
// (never advanced client-side -- the server is the source of truth for what
// counts as "after"), and the poll interval is floored at 1 second so a
// caller passing 0 (or any sub-second value) can't busy-loop the card.
func TestWaitForTap_ThreadsAfterAndPaces(t *testing.T) {
	const cardID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const cursor = "cursor-xyz"

	var (
		calls     []time.Time
		afterSeen []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, time.Now())
		afterSeen = append(afterSeen, r.URL.Query().Get("after"))
		w.Header().Set("Content-Type", "application/json")
		if len(calls) < 2 {
			w.Write(cardResourceFixture(cardID, []map[string]any{}))
			return
		}
		w.Write(cardResourceFixture(cardID, []map[string]any{
			{
				"id":         "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
				"user_id":    "cccccccc-cccc-cccc-cccc-cccccccccccc",
				"action_id":  "yes",
				"created_at": "2026-09-27T00:00:01Z",
			},
		}))
	}))
	defer srv.Close()

	client, err := saltapp.New("test-key", saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	tap, err := client.WaitForTap(context.Background(), cardID, cursor, 0 /* floored to 1s */)
	if err != nil {
		t.Fatalf("WaitForTap: %v", err)
	}
	elapsed := time.Since(start)

	if tap.ActionID != "yes" {
		t.Errorf("tap.ActionID = %q, want %q", tap.ActionID, "yes")
	}
	if len(calls) != 2 {
		t.Fatalf("GetCard called %d times, want 2", len(calls))
	}
	for i, a := range afterSeen {
		if a != cursor {
			t.Errorf("call %d: after = %q, want %q (unchanged cursor)", i, a, cursor)
		}
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, want >= ~1s (poll floor of 1s between the two calls)", elapsed)
	}
}

// TestWaitForTap_StopsOnContextDone proves the 1-second floor is real: with a
// context deadline shorter than the floor, WaitForTap must return after
// exactly one GetCard call (the deadline fires while it is waiting for the
// second, floored poll) rather than spinning.
func TestWaitForTap_StopsOnContextDone(t *testing.T) {
	const cardID = "dddddddd-dddd-dddd-dddd-dddddddddddd"

	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(cardResourceFixture(cardID, []map[string]any{}))
	}))
	defer srv.Close()

	client, err := saltapp.New("test-key", saltapp.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err = client.WaitForTap(ctx, cardID, "", 0)
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if callCount != 1 {
		t.Errorf("GetCard called %d times, want 1 (floor kept it from polling again before the deadline)", callCount)
	}
}
