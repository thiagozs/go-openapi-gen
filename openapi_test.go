package openapi

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thiagozs/go-openapi-gen/analyzer"
	"github.com/thiagozs/go-openapi-gen/integration"
	"github.com/thiagozs/go-openapi-gen/integration/testfixtures"
	"github.com/thiagozs/go-openapi-gen/parser"
	"github.com/thiagozs/go-openapi-gen/spec"
)

func TestBearerSecuritySchemeOmitsOAuthFlows(t *testing.T) {
	generator := &Generator{}
	data, err := json.Marshal(generator.generateSecuritySchemes()["bearerAuth"])
	require.NoError(t, err)

	var scheme map[string]any
	require.NoError(t, json.Unmarshal(data, &scheme))
	assert.Equal(t, "http", scheme["type"])
	assert.Equal(t, "bearer", scheme["scheme"])
	assert.NotContains(t, scheme, "flows")
}

func TestOAuthFlowsOnlySerializeConfiguredFlow(t *testing.T) {
	scheme := spec.SecurityScheme{
		Type: "oauth2",
		Flows: spec.OAuthFlows{
			ClientCredentials: spec.OAuthFlow{TokenURL: "https://example.com/token"},
		},
	}
	data, err := json.Marshal(scheme)
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	flows, ok := document["flows"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, flows, "clientCredentials")
	assert.NotContains(t, flows, "implicit")
	clientCredentials, ok := flows["clientCredentials"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{}, clientCredentials["scopes"])
}

func TestParameterizedRoutesGenerateDistinctOperationIDs(t *testing.T) {
	generator := &Generator{pathParser: parser.NewPathParser()}

	assert.Equal(t, "GetAdminTenants", generator.generateOperationID("GET", "/admin/v1/tenants"))
	assert.Equal(t, "GetAdminTenantsById", generator.generateOperationID("GET", "/admin/v1/tenants/:id"))
	assert.Equal(t, "GetPaymentsByTenantAndId", generator.generateOperationID("GET", "/v1/:tenant/payments/:id"))
}

func TestConfigControlsRuntimeASTAnalysis(t *testing.T) {
	development := NewDevelopmentConfig()
	assert.False(t, development.IsProductionMode())
	assert.True(t, development.IsASTAnalysisEnabled())

	production := NewProductionConfig()
	assert.True(t, production.IsProductionMode())
	production.DisableASTAnalysis = true
	assert.False(t, production.IsASTAnalysisEnabled())
}

func TestFallbackRequestSchemaOnlyAppliesToMethodsWithBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := func(c *gin.Context) {}
	engine.GET("/generic-get", handler)
	engine.POST("/generic-post", handler)

	options := &Options{}
	config := NewProductionConfig()
	config.SchemaDir = ""
	WithConfig(config)(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)

	document, err := generator.GenerateSpec()
	require.NoError(t, err)

	assert.NotContains(t, document.Components.Schemas, "GET_generic-getrequest")
	assert.Contains(t, document.Components.Schemas, "POST_generic-postrequest")
	assert.NotNil(t, document.Paths["/generic-get"].Get)
	assert.Nil(t, document.Paths["/generic-get"].Get.RequestBody)
	assert.NotNil(t, document.Paths["/generic-post"].Post)
	assert.NotNil(t, document.Paths["/generic-post"].Post.RequestBody)
}

func TestNewGeneratorSelectsGinAnalyzer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/payments", (&testfixtures.PaymentHandler{}).Create)

	options := &Options{}
	WithConfig(NewDevelopmentConfig())(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	assert.NoError(t, err)
	assert.IsType(t, &integration.GinHandlerAnalyzer{}, generator.handlerAnalyzer)

	document, err := generator.GenerateSpec()
	assert.NoError(t, err)
	requestSchema, ok := document.Components.Schemas["POST_paymentsrequest"]
	assert.True(t, ok)
	assert.Contains(t, requestSchema.Properties, "amount")
	responseSchema, ok := document.Components.Schemas["POST_paymentsresponse"]
	assert.True(t, ok)
	assert.Contains(t, responseSchema.Properties, "id")
}

func TestGeneratedSchemaIsUsedWithoutSourceAnalysis(t *testing.T) {
	generated := analyzer.HandlerSchema{
		RequestSchema: spec.Schema{Type: "object", Properties: map[string]spec.Schema{
			"amount":        {Type: "integer"},
			"compiled_only": {Type: "boolean"},
		}},
		ResponseSchema: spec.Schema{Type: "object", Properties: map[string]spec.Schema{
			"id": {Type: "string"},
		}},
	}
	analyzer.RegisterGeneratedHandlerSchema(
		"github.com/thiagozs/go-openapi-gen/integration/testfixtures.PaymentHandler.Create",
		generated,
	)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/compiled-payments", (&testfixtures.PaymentHandler{}).Create)
	options := &Options{}
	config := NewProductionConfig()
	config.SchemaDir = ""
	WithConfig(config)(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)

	document, err := generator.GenerateSpec()
	require.NoError(t, err)
	requestSchema := document.Components.Schemas["POST_compiled-paymentsrequest"]
	assert.Contains(t, requestSchema.Properties, "compiled_only")
}

func TestRouteSchemaOverride(t *testing.T) {
	om := NewOverrideManager()
	requestSchema := spec.Schema{Type: "object", Properties: map[string]spec.Schema{"amount": {Type: "number"}}}
	om.Override("POST", "/payments", RouteMetadata{RequestSchema: &requestSchema})

	metadata := om.GetMetadata("POST", "/payments", parser.NewPathParser().ParseRoute("POST", "/payments"))
	assert.Equal(t, requestSchema, *metadata.RequestSchema)
}

type testLogger struct{}

func (*testLogger) Info(string, ...any)  {}
func (*testLogger) Error(string, ...any) {}
func (*testLogger) Warn(string, ...any)  {}
func (*testLogger) Debug(string, ...any) {}

func TestPathParser(t *testing.T) {
	parser := parser.NewPathParser()

	tests := []struct {
		name            string
		method          string
		path            string
		expectedTag     string
		expectedSummary string
	}{
		{
			name:            "Auth login endpoint",
			method:          "POST",
			path:            "/api/v1/auth/login",
			expectedTag:     "auth",
			expectedSummary: "Create Auth Login",
		},
		{
			name:            "OAuth providers endpoint",
			method:          "GET",
			path:            "/api/v1/oauth/providers",
			expectedTag:     "oauth",
			expectedSummary: "Get Oauth Providers",
		},
		{
			name:            "Health check endpoint",
			method:          "GET",
			path:            "/health",
			expectedTag:     "health",
			expectedSummary: "Get Health",
		},
		{
			name:            "MFA setup endpoint",
			method:          "POST",
			path:            "/api/v1/user/mfa/setup",
			expectedTag:     "user",
			expectedSummary: "Create User Mfa Setup",
		},
		{
			name:            "Root endpoint",
			method:          "GET",
			path:            "/",
			expectedTag:     "root",
			expectedSummary: "Get Root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := parser.ParseRoute(tt.method, tt.path)

			assert.Equal(t, tt.expectedTag, parsed.Tag)
			assert.Equal(t, tt.expectedSummary, parsed.Summary)
			assert.Contains(t, parsed.Description, "operation")
		})
	}
}

func TestOverrideManager(t *testing.T) {
	om := NewOverrideManager()
	parser := parser.NewPathParser()

	// Test exact path override
	om.Override("POST", "/api/v1/auth/login", RouteMetadata{
		Tags:        "authentication",
		Summary:     "User Authentication",
		Description: "Authenticate user and return tokens",
	})

	// Parse route algorithmically
	parsed := parser.ParseRoute("POST", "/api/v1/auth/login")

	// Get metadata with overrides
	metadata := om.GetMetadata("POST", "/api/v1/auth/login", parsed)

	assert.Equal(t, "authentication", metadata.Tags)
	assert.Equal(t, "User Authentication", metadata.Summary)
	assert.Equal(t, "Authenticate user and return tokens", metadata.Description)
}

func TestPatternOverride(t *testing.T) {
	om := NewOverrideManager()
	parser := parser.NewPathParser()

	// Test pattern-based override
	err := om.OverridePattern("POST */login", RouteMetadata{
		Summary:     "Login Operation",
		Description: "Generic login operation",
	})
	assert.NoError(t, err)

	// Test different login endpoints
	loginPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/oauth/login",
		"/admin/login",
	}

	for _, path := range loginPaths {
		parsed := parser.ParseRoute("POST", path)
		metadata := om.GetMetadata("POST", path, parsed)

		// Pattern override should work now
		assert.Equal(t, "Login Operation", metadata.Summary)
		assert.Equal(t, "Generic login operation", metadata.Description)
	}
}

func TestTagOverrides(t *testing.T) {
	om := NewOverrideManager()
	parser := parser.NewPathParser()

	// Override auth tag
	om.OverrideTags("auth", "authentication")

	parsed := parser.ParseRoute("POST", "/api/v1/auth/login")
	metadata := om.GetMetadata("POST", "/api/v1/auth/login", parsed)

	assert.Equal(t, "authentication", metadata.Tags)
}
