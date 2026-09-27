// Command smoke is a live, mostly-read-only check against production
// saltapp.ai. It is not part of the saltapp module's public API (this
// directory is excluded from `go get`'s importable surface only insofar as
// nobody would import a main package) -- it exists to prove the generated
// client and the hand-written layer actually work against the real API, not
// just against httptest fixtures.
//
// It does three things:
//  1. Two public, unauthenticated, read-only calls (GET /api/v1/config,
//     GET /api/v1/agents/salt/agent-card.json) through the generated client
//     directly.
//  2. One authenticated round trip: register a throwaway root agent via
//     saltapp.RegisterAgent, then GET /api/v1/chats with its api-key (not
//     wrapped by the hand-written layer yet, so this reaches the generated
//     client the way any other caller needing an unwrapped endpoint would).
//  3. Prints the throwaway account's username on its own SMOKE_USERNAME=...
//     line and stops. It never purges the account itself -- that is a
//     salt-deploy operation run separately, against production, by a human
//     or agent who already holds that access. It never prints the api key.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/0000F8/saltapp-go"
	api "github.com/0000F8/saltapp-go/internal/saltapp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "smoke: FAILED:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	anon, err := api.NewClientWithResponses(saltapp.DefaultBaseURL)
	if err != nil {
		return fmt.Errorf("building read-only client: %w", err)
	}

	fmt.Println("== read-only checks ==")

	cfgResp, err := anon.GetConfigWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("GET /api/v1/config: %w", err)
	}
	if cfgResp.HTTPResponse.StatusCode != http.StatusOK || cfgResp.JSON200 == nil {
		return fmt.Errorf("GET /api/v1/config: unexpected status %d, body: %s", cfgResp.HTTPResponse.StatusCode, string(cfgResp.Body))
	}
	cfg := cfgResp.JSON200
	termsVersion := strOr(cfg.TermsVersion)
	privacyVersion := strOr(cfg.PrivacyVersion)
	fmt.Printf("GET /api/v1/config: ok (version=%s terms_version=%s privacy_version=%s)\n",
		strOr(cfg.Version), termsVersion, privacyVersion)

	cardResp, err := anon.GetAgentCardWithResponse(ctx, "salt")
	if err != nil {
		return fmt.Errorf("GET /api/v1/agents/salt/agent-card.json: %w", err)
	}
	if cardResp.HTTPResponse.StatusCode != http.StatusOK || cardResp.JSON200 == nil {
		return fmt.Errorf("GET /api/v1/agents/salt/agent-card.json: unexpected status %d, body: %s", cardResp.HTTPResponse.StatusCode, string(cardResp.Body))
	}
	fmt.Printf("GET /api/v1/agents/salt/agent-card.json: ok (%d top-level fields)\n", len(*cardResp.JSON200))

	fmt.Println("== authenticated round trip ==")

	suffix, err := randomHex(3)
	if err != nil {
		return fmt.Errorf("generating a random suffix: %w", err)
	}
	username := "SALT-go-smoke-" + suffix
	displayName := "Go smoke " + suffix // display names must not start with "salt"

	pubKey, cleanupKey, err := generateEphemeralPGPPublicKey(suffix)
	if err != nil {
		return fmt.Errorf("generating a throwaway PGP key: %w", err)
	}
	defer cleanupKey()

	fmt.Printf("registering %s ...\n", username)
	agent, err := saltapp.RegisterAgent(ctx, saltapp.RegisterAgentParams{
		Username:               username,
		DisplayName:            displayName,
		PublicKey:              pubKey,
		AcceptedTermsVersion:   termsVersion,
		AcceptedPrivacyVersion: privacyVersion,
	})
	if err != nil {
		fmt.Println("registration refused (a feature flag or a rate limit, not necessarily a bug):", err)
		fmt.Println("smoke stops here; read-only so far, nothing to purge.")
		return nil
	}
	if agent.APIKey == "" {
		return fmt.Errorf("registration succeeded but no api_key was returned")
	}
	fmt.Printf("registered: id=%s username=%s account_type=%s (api key received, not printed)\n",
		agent.ID, agent.Username, agent.AccountType)

	authed, err := api.NewClientWithResponses(saltapp.DefaultBaseURL, api.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("api-key", agent.APIKey)
		return nil
	}))
	if err != nil {
		return fmt.Errorf("building authenticated client: %w", err)
	}

	chatsResp, err := authed.ListChatsWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("GET /api/v1/chats: %w", err)
	}
	numChats := 0
	if chatsResp.JSON200 != nil {
		numChats = len(*chatsResp.JSON200)
	}
	fmt.Printf("GET /api/v1/chats: status=%d chats=%d\n", chatsResp.HTTPResponse.StatusCode, numChats)

	// Machine-parseable, so whatever purges this account afterward doesn't
	// have to scrape the sentence above.
	fmt.Printf("SMOKE_USERNAME=%s\n", username)
	return nil
}

func strOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateEphemeralPGPPublicKey shells out to gpg (this SDK has no PGP
// support of its own -- see the package doc) to create a throwaway RSA-2048
// keypair in a scratch GNUPGHOME, used only to satisfy POST /auth's required
// public_key for this one smoke run. The private half is discarded with the
// temp directory; nothing durable is written.
func generateEphemeralPGPPublicKey(suffix string) (armoredPublicKey string, cleanup func(), err error) {
	home, err := os.MkdirTemp("", "saltapp-go-smoke-gnupg-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(home) }

	email := fmt.Sprintf("salt-go-smoke-%s@example.test", suffix)
	batch := fmt.Sprintf(`%%no-protection
Key-Type: RSA
Key-Length: 2048
Subkey-Type: RSA
Subkey-Length: 2048
Name-Real: Salt Go Smoke
Name-Email: %s
Expire-Date: 1d
%%commit
`, email)
	batchPath := filepath.Join(home, "batch")
	if err := os.WriteFile(batchPath, []byte(batch), 0o600); err != nil {
		cleanup()
		return "", nil, err
	}

	gen := exec.Command("gpg", "--homedir", home, "--batch", "--gen-key", batchPath)
	if out, err := gen.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("gpg --gen-key: %w\n%s", err, out)
	}

	exp := exec.Command("gpg", "--homedir", home, "--batch", "--armor", "--export", email)
	out, err := exp.Output()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("gpg --export: %w", err)
	}
	return string(out), cleanup, nil
}
