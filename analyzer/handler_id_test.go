package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeHandlerID(t *testing.T) {
	assert.Equal(t,
		"github.com/acme/payments.PaymentHandler.Create",
		NormalizeHandlerID("github.com/acme/payments.(*PaymentHandler).Create-fm"),
	)
	assert.Equal(t,
		"github.com/acme/payments.Health",
		NormalizeHandlerID("github.com/acme/payments.Health"),
	)
}
