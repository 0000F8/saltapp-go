package saltapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	api "github.com/0000F8/saltapp-go/internal/saltapp"
)

// Card block "type" and button "style"/"action_type" values, mirroring the
// OpenAPI document's enums (CardBlock, CardBlock.Elements).
const (
	CardBlockTypeSection = "section"
	CardBlockTypeImage   = "image"
	CardBlockTypeDivider = "divider"
	CardBlockTypeActions = "actions"

	ButtonStylePrimary = "primary"
	ButtonStyleDanger  = "danger"

	ActionTypeDefault = "default"
	ActionTypePay     = "pay"
	ActionTypeHandoff = "handoff"
)

// CardField is one label/value pair inside a section block.
type CardField struct {
	Label string
	Value string
}

// CardButtonPay is the amount a "pay" button's tap turns into a real
// TransferRequest on Salt's existing payment rail.
type CardButtonPay struct {
	// Amount is a plain human-decimal string, e.g. "1.50" -- never a float,
	// never scientific notation.
	Amount   string
	Currency string
}

// CardButton is one button inside an actions block (1-5 per block).
type CardButton struct {
	// ActionID identifies the button in a card_interaction and must match
	// ^[a-z0-9_-]{1,40}$.
	ActionID string
	Label    string

	// Style is "", ButtonStylePrimary, or ButtonStyleDanger.
	Style string

	// ActionType is "" (an ordinary tap, forwarded to the owner's webhook as
	// a card_interaction), ActionTypePay, or ActionTypeHandoff.
	ActionType string

	// Pay is required when ActionType is ActionTypePay.
	Pay *CardButtonPay

	// HandoffTargetAgentID is required when ActionType is ActionTypeHandoff.
	HandoffTargetAgentID string

	// RestrictedTo, if non-empty, limits who may tap this button to these
	// user/agent ids (max 20).
	RestrictedTo []string
}

// Button is a plain button: a tap forwards to the owner's webhook as a
// card_interaction (the usual "ask a question" shape for WaitForTap).
func Button(actionID, label string) CardButton {
	return CardButton{ActionID: actionID, Label: label}
}

// PayButton is a button whose tap creates a real TransferRequest for amount
// (a plain decimal string, e.g. "1.50") in currency.
func PayButton(actionID, label, amount, currency string) CardButton {
	return CardButton{
		ActionID:   actionID,
		Label:      label,
		ActionType: ActionTypePay,
		Pay:        &CardButtonPay{Amount: amount, Currency: currency},
	}
}

// CardBlock is one block of a card's declarative UI (at most 20 per card).
// Build one with SectionBlock, ImageBlock, DividerBlock, or ActionsBlock
// rather than setting Type by hand.
type CardBlock struct {
	Type     string
	Text     string
	Fields   []CardField
	URL      string
	Alt      string
	Elements []CardButton
}

// SectionBlock is a block of text, optionally with label/value fields.
func SectionBlock(text string, fields ...CardField) CardBlock {
	return CardBlock{Type: CardBlockTypeSection, Text: text, Fields: fields}
}

// ImageBlock is an image block. url must be http(s) and at most 500 chars.
func ImageBlock(url, alt string) CardBlock {
	return CardBlock{Type: CardBlockTypeImage, URL: url, Alt: alt}
}

// DividerBlock is a plain visual divider.
func DividerBlock() CardBlock {
	return CardBlock{Type: CardBlockTypeDivider}
}

// ActionsBlock is a row of 1-5 buttons.
func ActionsBlock(buttons ...CardButton) CardBlock {
	return CardBlock{Type: CardBlockTypeActions, Elements: buttons}
}

func toGenBlocks(blocks []CardBlock) ([]api.CardBlock, error) {
	out := make([]api.CardBlock, 0, len(blocks))
	for _, b := range blocks {
		gb := api.CardBlock{Type: api.CardBlockType(b.Type)}
		if b.Text != "" {
			text := b.Text
			gb.Text = &text
		}
		if b.URL != "" {
			url := b.URL
			gb.Url = &url
		}
		if b.Alt != "" {
			alt := b.Alt
			gb.Alt = &alt
		}
		if len(b.Fields) > 0 {
			fields := make([]struct {
				Label *string `json:"label,omitempty"`
				Value *string `json:"value,omitempty"`
			}, len(b.Fields))
			for i, f := range b.Fields {
				label, value := f.Label, f.Value
				fields[i].Label = &label
				fields[i].Value = &value
			}
			gb.Fields = &fields
		}
		if len(b.Elements) > 0 {
			elements := make([]struct {
				ActionId             *string                          `json:"action_id,omitempty"`
				ActionType           *api.CardBlockElementsActionType `json:"action_type,omitempty"`
				HandoffTargetAgentId *uuid.UUID                       `json:"handoff_target_agent_id,omitempty"`
				Label                *string                          `json:"label,omitempty"`
				Pay                  *struct {
					Amount   *string `json:"amount,omitempty"`
					Currency *string `json:"currency,omitempty"`
				} `json:"pay,omitempty"`
				RestrictedTo *[]uuid.UUID                `json:"restricted_to,omitempty"`
				Style        *api.CardBlockElementsStyle `json:"style,omitempty"`
				Type         *api.CardBlockElementsType  `json:"type,omitempty"`
			}, len(b.Elements))
			for i, btn := range b.Elements {
				actionID, label := btn.ActionID, btn.Label
				elements[i].ActionId = &actionID
				elements[i].Label = &label
				buttonType := api.CardBlockElementsTypeButton
				elements[i].Type = &buttonType
				if btn.Style != "" {
					style := api.CardBlockElementsStyle(btn.Style)
					elements[i].Style = &style
				}
				if btn.ActionType != "" {
					actionType := api.CardBlockElementsActionType(btn.ActionType)
					elements[i].ActionType = &actionType
				}
				if btn.Pay != nil {
					amount, currency := btn.Pay.Amount, btn.Pay.Currency
					elements[i].Pay = &struct {
						Amount   *string `json:"amount,omitempty"`
						Currency *string `json:"currency,omitempty"`
					}{Amount: &amount, Currency: &currency}
				}
				if btn.HandoffTargetAgentID != "" {
					id, err := uuid.Parse(btn.HandoffTargetAgentID)
					if err != nil {
						return nil, fmt.Errorf("saltapp: button %q handoff_target_agent_id: %w", btn.ActionID, err)
					}
					elements[i].HandoffTargetAgentId = &id
				}
				if len(btn.RestrictedTo) > 0 {
					ids := make([]uuid.UUID, len(btn.RestrictedTo))
					for j, s := range btn.RestrictedTo {
						id, err := uuid.Parse(s)
						if err != nil {
							return nil, fmt.Errorf("saltapp: button %q restricted_to[%d]: %w", btn.ActionID, j, err)
						}
						ids[j] = id
					}
					elements[i].RestrictedTo = &ids
				}
			}
			gb.Elements = &elements
		}
		out = append(out, gb)
	}
	return out, nil
}

// CardOwner is the small user/agent summary a Card names its owner with.
type CardOwner struct {
	ID          string
	Username    string
	DisplayName string
	AccountType string
	System      bool
}

// CardInteraction is one recorded tap on a card's button (card_interactions
// row). A "pay" button's tap carries the resulting TransferRequest's id and
// its live status in TransferRequestID/TransferRequestStatus.
type CardInteraction struct {
	ID                    string
	UserID                string
	ActionID              string
	Value                 string
	CreatedAt             time.Time
	TransferRequestID     string
	TransferRequestStatus string
}

// Card is a card's current state plus its interaction history, as returned
// by GetCard (owner-only).
type Card struct {
	ID           string
	Owner        CardOwner
	Interactions []CardInteraction
}

func fromGenCard(cr *api.CardResource) *Card {
	if cr == nil {
		return &Card{}
	}
	c := &Card{}
	if cr.Id != nil {
		c.ID = cr.Id.String()
	}
	if cr.Owner != nil {
		c.Owner = CardOwner{
			ID:          cr.Owner.Id.String(),
			Username:    cr.Owner.Username,
			DisplayName: cr.Owner.DisplayName,
			AccountType: string(cr.Owner.AccountType),
		}
		if cr.Owner.System != nil {
			c.Owner.System = *cr.Owner.System
		}
	}
	if cr.Interactions != nil {
		c.Interactions = make([]CardInteraction, len(*cr.Interactions))
		for i, in := range *cr.Interactions {
			ci := CardInteraction{}
			if in.Id != nil {
				ci.ID = in.Id.String()
			}
			if in.UserId != nil {
				ci.UserID = in.UserId.String()
			}
			if in.ActionId != nil {
				ci.ActionID = *in.ActionId
			}
			if in.Value != nil {
				ci.Value = *in.Value
			}
			if in.CreatedAt != nil {
				ci.CreatedAt = *in.CreatedAt
			}
			if in.TransferRequestId != nil {
				ci.TransferRequestID = in.TransferRequestId.String()
			}
			if in.TransferRequestStatus != nil {
				ci.TransferRequestStatus = *in.TransferRequestStatus
			}
			c.Interactions[i] = ci
		}
	}
	return c
}

// PostCard posts an interactive card into chatID: blocks is its declarative
// UI (build them with SectionBlock/ImageBlock/DividerBlock/ActionsBlock), and
// text is the plaintext preview shown wherever the card itself can't render
// (truncated to 200 chars server-side). It returns the new card's id and the
// chat message's id -- note there is NO top-level "id" in the response; the
// OpenAPI document names these resource_id and message_id.
//
// A card carrying a "pay" button (see PayButton) requires an api-key (an
// OAuth bearer token needs the "money" scope too, which this SDK does not
// yet support for card creation).
func (c *Client) PostCard(ctx context.Context, chatID, text string, blocks []CardBlock) (cardID, messageID string, err error) {
	genBlocks, err := toGenBlocks(blocks)
	if err != nil {
		return "", "", err
	}
	body := api.CreateCardJSONRequestBody{
		ChatId: chatID,
		Blocks: genBlocks,
	}
	if text != "" {
		body.Text = &text
	}

	resp, err := c.gen.CreateCardWithResponse(ctx, body)
	if err != nil {
		return "", "", err
	}
	switch {
	case resp.JSON200 != nil:
		msg := resp.JSON200
		if msg.ResourceId == nil {
			return "", "", errors.New("saltapp: card created but the response carried no resource_id")
		}
		mid := msg.MessageId.String()
		return *msg.ResourceId, mid, nil
	case resp.JSON401 != nil:
		return "", "", singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON401)
	case resp.JSON422 != nil:
		return "", "", pluralAPIError(resp.HTTPResponse.StatusCode, resp.JSON422)
	default:
		return "", "", unexpectedStatus(resp.HTTPResponse.StatusCode, resp.Body)
	}
}

// GetCard reads a card's current state and interaction history (owner only).
// after, if non-empty, is a CardInteraction id or an ISO 8601 timestamp: only
// interactions after it are returned (unparseable fails open to everything).
// An unknown id and a real-but-not-yours card render the same 404.
func (c *Client) GetCard(ctx context.Context, cardID, after string) (*Card, error) {
	var params *api.GetCardParams
	if after != "" {
		params = &api.GetCardParams{After: &after}
	}
	resp, err := c.gen.GetCardWithResponse(ctx, cardID, params)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.JSON200 != nil:
		return fromGenCard(resp.JSON200), nil
	case resp.JSON401 != nil:
		return nil, singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON401)
	case resp.JSON404 != nil:
		return nil, singleAPIError(resp.HTTPResponse.StatusCode, resp.JSON404)
	default:
		return nil, unexpectedStatus(resp.HTTPResponse.StatusCode, resp.Body)
	}
}

// WaitForTap polls GetCard(ctx, cardID, after) until an interaction appears
// past the after cursor, or ctx is done, and returns the earliest such
// interaction. poll is the interval between reads and is floored at 1
// second.
//
// This is the ask flow every Go agent host needs: PostCard, then
// WaitForTap. It deliberately never touches GET /api/v1/agent/updates --
// that outbox holds exactly one forward-only cursor per agent, shared with
// every other consumer of that agent's socket-mode deliveries, and polling
// it from inside a single tool call is the exact bug class Salt removed
// everywhere in the run that produced this SDK (one poller per agent, ever).
// A card's own interaction history has no such shared cursor: each caller's
// own `after` is its own.
func (c *Client) WaitForTap(ctx context.Context, cardID, after string, poll time.Duration) (*CardInteraction, error) {
	if poll < time.Second {
		poll = time.Second
	}
	for {
		card, err := c.GetCard(ctx, cardID, after)
		if err != nil {
			return nil, err
		}
		if len(card.Interactions) > 0 {
			tap := card.Interactions[0]
			return &tap, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(poll):
		}
	}
}
