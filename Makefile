.PHONY: generate test vet lint fmt smoke

# Regenerate internal/saltapp/{types,client}.gen.go from openapi.json.
# See AGENTS.md for what to do when oapi-codegen can't consume a new spec construct.
generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config oapi-codegen.types.yaml openapi.json
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config oapi-codegen.client.yaml openapi.json

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

# Read-only + one throw-away-account round trip against production. See smoke/main.go.
smoke:
	go run ./smoke
