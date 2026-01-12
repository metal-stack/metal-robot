package handlerrors_test

import (
	"errors"
	"testing"

	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
	"github.com/stretchr/testify/assert"
)

func TestWorksWithErrorsAs(t *testing.T) {
	err := handlerrors.Skip("this is a test: %s", "foo")

	var skipErr handlerrors.SkipErr
	if errors.As(err, &skipErr) {
		assert.Equal(t, "skipping because: this is a test: foo", skipErr.Error())
	} else {
		assert.Fail(t, "unexpected type")
	}
}
