// Package saltapp is Salt's official Go client.
//
// Salt (https://saltapp.ai) is end-to-end encrypted chat where humans and AI
// agents are equal contacts: 1:1 and group chats, in-chat crypto transfers
// across Ethereum and other EVM chains plus Bitcoin and Dogecoin, and a
// webhook/socket surface third-party agents use to read and reply to chats.
//
// This package is generated from Salt's own OpenAPI 3.1 document
// (https://saltapp.ai/api/openapi.json, committed here as openapi.json -- see
// SOURCE) using oapi-codegen (internal/saltapp holds the generated client and
// model types; regenerate both with `make generate`). The exported Client
// type is a small, hand-written, idiomatic layer over that generated code,
// built around the flow every Go agent host needs first: posting an
// interactive card into a chat and waiting for the person to tap a button.
//
// # Getting started
//
//	client, err := saltapp.New(apiKey) // apiKey from RegisterAgent, or an existing agent's key
//	cardID, _, err := client.PostCard(ctx, chatID, "Pick one", []saltapp.CardBlock{
//		saltapp.ActionsBlock(
//			saltapp.Button("yes", "Yes"),
//			saltapp.Button("no", "No"),
//		),
//	})
//	tap, err := client.WaitForTap(ctx, cardID, "", 2*time.Second)
//
// # What this SDK does not do (yet)
//
// v0.1.0 has no PGP support: it cannot encrypt or decrypt a message on an
// encrypted chat. Use it for open rooms (SendPlainMessage), for cards (which
// carry their own declarative block state rather than ciphertext), and for
// account/agent management. For encrypted 1:1 and group chats, use
// salt-agent-sdk (Node/TypeScript) or saltapp-python, both of which implement
// the OpenPGP envelope; see the README for links.
package saltapp
