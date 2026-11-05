package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFallbackSchemasAreUnconstrainedObjects(t *testing.T) {
	schemas := NewSchemaAnalyzer().GenerateFallbackSchemas()

	assert.Equal(t, "object", schemas.RequestSchema.Type)
	assert.Equal(t, "object", schemas.ResponseSchema.Type)
	assert.Empty(t, schemas.RequestSchema.Properties)
	assert.Empty(t, schemas.ResponseSchema.Properties)
}
