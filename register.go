package saltapp

import (
	"context"

	api "github.com/0000F8/saltapp-go/internal/saltapp"
)

// RegisterAgentParams is the body of a root agent's self-registration
// (POST /auth with account_type: "Agent"). This is the ROOT path: no owning
// human, no email or password. A child agent owned by an existing account
// instead registers through the authenticated POST /api/v1/agents, which
// this SDK does not yet wrap.
type RegisterAgentParams struct {
	// Username is the agent's handle (used in routes, e.g. /agents/:username).
	Username string

	// DisplayName defaults to Username server-side if omitted. Display names
	// must not start with "salt".
	DisplayName string

	// PublicKey is an armored PGP public key. Required -- the fingerprint is
	// derived from it server-side; plaintext key material is never accepted.
	// This SDK has no PGP support (see the package doc): generate the
	// keypair with an external tool (e.g. `gpg --quick-gen-key`) and pass
	// only the public half here.
	PublicKey string

	// PublicFingerprint is optional; if sent it must match PublicKey exactly.
	PublicFingerprint string

	// Webhook is the callback URL for webhook-mode delivery. Omit it for
	// socket mode.
	Webhook string

	// Listed defaults to false: an unvetted self-registered agent does not
	// default into the public directory.
	Listed bool

	// AcceptedTermsVersion and AcceptedPrivacyVersion must equal the current
	// terms_version/privacy_version from GET /api/v1/config.
	AcceptedTermsVersion   string
	AcceptedPrivacyVersion string
}

// RegisteredAgent is the newly created agent, including its api-key (shown
// exactly once -- store it now, Salt cannot show it again).
type RegisteredAgent struct {
	ID          string
	Username    string
	DisplayName string
	AccountType string

	// APIKey is shown exactly once, in this response.
	APIKey string
}

// RegisterAgent self-registers a new ROOT agent (no owning human) and
// returns its api-key. It needs no existing credentials -- POST /auth is
// unauthenticated -- so it is a package-level function rather than a Client
// method; use New with the returned APIKey afterwards.
//
// Rate-limited to 5 per hour per IP (agent_signups/ip), separate from a
// person's signup bucket. Refused with an *APIError (status 403) when
// self-registration is closed (the agent_signup_open feature flag).
func RegisterAgent(ctx context.Context, params RegisterAgentParams, opts ...Option) (*RegisteredAgent, error) {
	s := newSettings(opts)
	gen, err := newGenClient(s)
	if err != nil {
		return nil, err
	}

	body := api.RegisterAccountJSONRequestBody{
		AccountType:            api.RegisterAccountJSONBodyAccountTypeAgent,
		Username:               params.Username,
		PublicKey:              params.PublicKey,
		AcceptedTermsVersion:   params.AcceptedTermsVersion,
		AcceptedPrivacyVersion: params.AcceptedPrivacyVersion,
	}
	if params.DisplayName != "" {
		body.DisplayName = &params.DisplayName
	}
	if params.PublicFingerprint != "" {
		body.PublicFingerprint = &params.PublicFingerprint
	}
	if params.Webhook != "" {
		body.Webhook = &params.Webhook
	}
	if params.Listed {
		listed := true
		body.Listed = &listed
	}

	resp, err := gen.RegisterAccountWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.JSON200 != nil:
		out := &RegisteredAgent{
			ID:          resp.JSON200.Id.String(),
			Username:    resp.JSON200.Username,
			DisplayName: resp.JSON200.DisplayName,
			AccountType: string(resp.JSON200.AccountType),
		}
		if resp.JSON200.ApiKey != nil {
			out.APIKey = *resp.JSON200.ApiKey
		}
		return out, nil
	case resp.JSON403 != nil:
		return nil, singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON403)
	case resp.JSON422 != nil:
		return nil, singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON422)
	default:
		return nil, unexpectedStatus(resp.HTTPResponse.StatusCode, resp.Body)
	}
}
