package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thiagozs/go-openapi-gen/spec"
)

func TestGeneratedHandlerSchemaIsCopiedIntoNewRegistry(t *testing.T) {
	generated := HandlerSchema{
		RequestSchema: spec.Schema{Type: "object", Properties: map[string]spec.Schema{
			"amount": {Type: "number"},
		}},
	}
	RegisterGeneratedHandlerSchema("GeneratedCreate", generated)

	registry := NewSchemaRegistry()
	actual, ok := registry.GetHandlerSchema("GeneratedCreate")
	require.True(t, ok)
	assert.Equal(t, generated, actual)
}
