package saltapp_test

import (
	"errors"

	"github.com/0000F8/saltapp-go"
)

// asAPIError is a tiny errors.As wrapper shared by the test files so each
// doesn't need to repeat the type-assertion boilerplate.
func asAPIError(err error, target **saltapp.APIError) bool {
	return errors.As(err, target)
}
