package saltapp

import (
	"context"
	"errors"
	"net/http"
	"strings"

	api "github.com/0000F8/saltapp-go/internal/saltapp"
)

// DefaultBaseURL is Salt's production API origin.
const DefaultBaseURL = "https://saltapp.ai"

// Client is a small, idiomatic wrapper around Salt's generated OpenAPI
// client. Construct one with New.
type Client struct {
	gen *api.ClientWithResponses
}

// Option configures a Client constructed by New, or the transient client
// RegisterAgent uses.
type Option func(*settings)

type settings struct {
	baseURL    string
	httpClient api.HttpRequestDoer
}

// WithBaseURL overrides the default API origin (https://saltapp.ai). Useful
// for a staging environment or a local salt-api instance.
func WithBaseURL(baseURL string) Option {
	return func(s *settings) {
		s.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client used to make requests (anything
// satisfying `Do(*http.Request) (*http.Response, error)`, including a plain
// *http.Client). Useful for custom transports, timeouts, or tests.
func WithHTTPClient(hc api.HttpRequestDoer) Option {
	return func(s *settings) {
		s.httpClient = hc
	}
}

func newSettings(opts []Option) *settings {
	s := &settings{baseURL: DefaultBaseURL}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func newGenClient(s *settings, editors ...api.RequestEditorFn) (*api.ClientWithResponses, error) {
	genOpts := make([]api.ClientOption, 0, len(editors)+1)
	for _, e := range editors {
		genOpts = append(genOpts, api.WithRequestEditorFn(e))
	}
	if s.httpClient != nil {
		genOpts = append(genOpts, api.WithHTTPClient(s.httpClient))
	}
	return api.NewClientWithResponses(s.baseURL, genOpts...)
}

// New constructs a Client authenticated with an agent's own api-key: from
// RegisterAgent, from POST /api/v1/agents for a child agent, or from Settings
// -> Developers -> API keys in the Salt app. Every request carries it in the
// `api-key` header, per the OpenAPI document's apiKey security scheme --
// never as a query parameter, which would land the key in access logs.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("saltapp: api key must not be empty")
	}

	s := newSettings(opts)
	gen, err := newGenClient(s, func(_ context.Context, req *http.Request) error {
		req.Header.Set("api-key", apiKey)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Client{gen: gen}, nil
}
