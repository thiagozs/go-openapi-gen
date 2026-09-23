package common

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackSchemasUseValidOpenAPITypes(t *testing.T) {
	schemas := NewSchemaAnalyzer().GenerateFallbackSchemas()

	requestJSON, err := json.Marshal(schemas.RequestSchema)
	require.NoError(t, err)
	responseJSON, err := json.Marshal(schemas.ResponseSchema)
	require.NoError(t, err)

	assert.False(t, strings.Contains(string(requestJSON), `"type":"any"`))
	assert.False(t, strings.Contains(string(responseJSON), `"type":"any"`))
	assert.Equal(t, "object", schemas.RequestSchema.Type)
	assert.Equal(t, "object", schemas.ResponseSchema.Type)
	assert.NotNil(t, schemas.RequestSchema.Properties["data"].AdditionalProperties)
	assert.NotNil(t, schemas.ResponseSchema.Properties["data"].AdditionalProperties)
}
