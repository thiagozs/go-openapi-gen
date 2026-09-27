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

func TestGeneratedHandlerSchemaRegisteredAfterRegistryCreationIsVisible(t *testing.T) {
	registry := NewSchemaRegistry()
	generated := HandlerSchema{
		RequestSchema: spec.Schema{Type: "object", Properties: map[string]spec.Schema{
			"email": {Type: "string"},
		}},
	}

	RegisterGeneratedHandlerSchema("LateGeneratedLogin", generated)

	actual, ok := registry.GetHandlerSchema("LateGeneratedLogin")
	require.True(t, ok)
	assert.Equal(t, generated, actual)
	assert.True(t, registry.HasHandlerSchema("LateGeneratedLogin"))
	assert.Contains(t, registry.GetAllHandlerNames(), "LateGeneratedLogin")
}
