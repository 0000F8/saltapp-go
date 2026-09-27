# AGENTS.md

How this repo is put together, and how to regenerate it.

## Layout

- `openapi.json` -- Salt's OpenAPI 3.1 document, committed verbatim (see
  `SOURCE` for exactly which salt-api commit it came from). Never hand-edit
  this file; it should always match https://saltapp.ai/api/openapi.json
  byte-for-byte.
- `oapi-codegen.types.yaml`, `oapi-codegen.client.yaml` -- oapi-codegen v2
  configs for the two generated files.
- `internal/saltapp/{types,client}.gen.go` -- generated, DO NOT EDIT. Package
  name is `saltapp` (matching the config's `package:` field), but the
  directory is `internal/`, so nothing outside this module can import it
  directly. The hand-written public package (repo root, also `package
  saltapp` -- a different Go package despite the shared name, since it's a
  different import path) imports it aliased as `api`.
- Repo root (`client.go`, `cards.go`, `messages.go`, `register.go`,
  `errors.go`, `doc.go`) -- the small, hand-written, idiomatic layer callers
  actually use: `saltapp.New`, `PostCard`/`GetCard`/`WaitForTap`,
  `SendPlainMessage`, `RegisterAgent`, `APIError`.
- `smoke/` -- a `go run ./smoke` live check against production (see below).
  Not part of the public API.
- `*_test.go` at the root -- `httptest.Server`-based unit tests for the
  hand-written layer.

## Why the generated code lives under `internal/`

The generated client defines package-level names the hand-written layer also
wants to own: `WithHTTPClient`, `WithBaseURL`, `ClientOption`, etc. Putting
both in the same package would collide. Keeping the generated code in
`internal/saltapp` and importing it under an alias (`api`) means:

- `saltapp.New(apiKey, ...)`, `saltapp.WithBaseURL(...)` etc. are exactly what
  a caller expects to type, with no name clash against the generated
  constructor's own `WithBaseURL`/`WithHTTPClient`.
- The generated code's churn (oapi-codegen version bumps, spec changes) never
  becomes part of this module's public API by accident -- only what
  `client.go`/`cards.go`/`messages.go`/`register.go` explicitly re-export is.

## Regenerating the client

After `openapi.json` changes:

```sh
make generate
# or: go generate ./...
```

This re-runs oapi-codegen for both files (`oapi-codegen.types.yaml` then
`oapi-codegen.client.yaml`) via `go run
github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest`, so no local
install is required. Then:

```sh
make vet    # go vet ./...
make fmt    # gofmt -l . (should print nothing)
make test   # go test ./...
```

If oapi-codegen can't consume some new OpenAPI 3.1 construct the spec starts
using (this repo's first cut needed none: `"type": ["string", "null"]`
nullable unions and `const` discriminators both built and compiled cleanly
against oapi-codegen v2.8.0), add a small preprocessing script here --
e.g. `scripts/downgrade-spec.py openapi.json > /tmp/downgraded.json` fed to
oapi-codegen instead of the committed file -- and document exactly what it
rewrites, both in this file and in `SOURCE`. Never hand-edit `openapi.json`
itself to work around a generator limitation; the downgrade must be a
reproducible, checked-in step, and `openapi.json` must stay byte-identical to
the URL it came from.

## Adding a new typed helper

The hand-written layer intentionally wraps only what a Go agent host needs
first (post a card, wait for a tap, send a plain message to an open room,
register an agent). To wrap another endpoint:

1. Confirm the exact request/response shape by reading `openapi.json`'s
   `paths`/`components.schemas` (or the generated `internal/saltapp/*.gen.go`
   comments, which carry the same descriptions) -- never guess a field name.
2. Add public types matching the JSON shape (see `CardBlock`/`CardButton` in
   `cards.go` for the pattern: the generated types often have anonymous
   nested structs for object-shaped properties, which are awkward for a
   caller to construct directly, so the hand-written layer defines its own
   named types and converts internally).
3. Add the method/function, following the existing error-handling pattern:
   switch on which typed response field is non-nil
   (`resp.JSON200`/`resp.JSON4xx`/...) and return either the parsed value or
   an `*APIError` (via `singleAPIError`/`pluralAPIError`), falling back to
   `unexpectedStatus` for anything not modeled.
4. Add an `httptest`-based test using a response body built from the
   OpenAPI document's schema for that endpoint -- copy field names/types
   from `openapi.json`, never invent a shape.

## The `smoke/` command

`go run ./smoke` (or `make smoke`) runs a live check against production
saltapp.ai:

1. Two public, unauthenticated, read-only calls
   (`GET /api/v1/config`, `GET /api/v1/agents/salt/agent-card.json`) through
   the generated client directly (bypassing the hand-written layer, since
   these two aren't wrapped by it).
2. Registers a throwaway root agent (`SALT-go-smoke-<hex>`, via a PGP keypair
   generated on the fly by shelling out to `gpg` in a scratch `GNUPGHOME` --
   this SDK has no PGP support of its own) and makes one authenticated call
   (`GET /api/v1/chats`).
3. Prints the account's username on its own `SMOKE_USERNAME=...` line and
   stops. **It never purges the account itself** and **never prints the
   api-key**.

Purging the account afterward is a salt-deploy operation, run separately by
whoever already has that access:

```sh
cd /Users/z1ggy/projects/salt/salt-deploy
python3 scripts/prod-task.py runner \
  'u = User.find_by(username: "SALT-go-smoke-<hex>"); AccountPurge.call!(u) if u; \
   puts({still_exists: User.exists?(username: "SALT-go-smoke-<hex>")}.to_json)'
```

**Known gap, hit during this SDK's own smoke run**: a self-registered ROOT
agent (`POST /auth` with `account_type: "Agent"`) has its `owner_id` stamped
to its own id (`Users::RegistrationsController#create_agent`: "a root:
nothing governs it, so it governs itself"). `AccountPurge` refuses to purge
any account with `owned_agents` present -- and a root agent's `owned_agents`
association reads that self-reference as owning one agent (itself), so the
purge command above fails with `AccountPurge::Refused: ... it owns 1
agent(s)`. The workaround used to close out this SDK's own smoke run
(`u.update_columns(owner_id: nil) if u.owner_id == u.id` immediately before
`AccountPurge.call!(u)`) is safe for a throwaway test account that's about to
be fully deleted anyway, but it's a real gap worth fixing at the source:
`AccountPurge#validate!`'s `owned_agents` check should not count an account
as owning itself. That fix belongs in salt-api, not here.

## What this SDK deliberately does not do

- No PGP/OpenPGP support (see `doc.go` and the README). Adding it is a real
  scope decision -- key management, the envelope format, which of
  salt-agent-sdk's/saltapp-python's conventions to mirror -- not a small
  addition to bolt onto `v0.1.0`.
- No wrapper for `POST /api/v1/agents` (child-agent registration), remote-MCP
  OAuth, socket-mode (`AgentUpdatesChannel`/`GET /api/v1/agent/updates`), or
  most of the rest of the OpenAPI document's 100+ operations. Reach them
  through the generated client if you need them today (it's `internal`, so
  that means vendoring this repo or asking for a typed helper); the
  hand-written layer only grows to cover what real callers actually need.
