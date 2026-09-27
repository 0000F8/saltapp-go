package saltapp

import (
	"context"

	api "github.com/0000F8/saltapp-go/internal/saltapp"
)

// SendPlainMessage posts a plain-text message into chatID and returns the
// new message's id.
//
// Salt's chats are end-to-end encrypted by default; this SDK has no PGP
// support (see the package doc), so SendPlainMessage only works on an OPEN
// room (a public, unencrypted group chat -- see GET /api/v1/chats/:id's
// `encrypted` field, or a room's own /rooms/:id page). It always sends
// `encrypted: false` explicitly: against a genuinely encrypted chat the API
// refuses this rather than silently accepting plaintext as if it were
// ciphertext, and that refusal surfaces here as an *APIError with status
// 422.
func (c *Client) SendPlainMessage(ctx context.Context, chatID, message string) (messageID string, err error) {
	encrypted := false
	body := api.CreateMessageJSONRequestBody{
		ChatId:    chatID,
		Message:   message,
		Encrypted: &encrypted,
	}

	resp, err := c.gen.CreateMessageWithResponse(ctx, body)
	if err != nil {
		return "", err
	}
	switch {
	case resp.JSON200 != nil:
		return resp.JSON200.MessageId.String(), nil
	case resp.JSON401 != nil:
		return "", singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON401)
	case resp.JSON403 != nil:
		return "", singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON403)
	case resp.JSON422 != nil:
		return "", singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON422)
	case resp.JSON429 != nil:
		return "", singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON429)
	default:
		return "", unexpectedStatus(resp.HTTPResponse.StatusCode, resp.Body)
	}
}
