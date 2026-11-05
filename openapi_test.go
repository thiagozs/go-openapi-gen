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
	assert.Contains(t, document.Paths["/payments"].Post.Responses, "201")
	assert.NotContains(t, document.Paths["/payments"].Post.Responses, "200")
}

func TestDeleteNoContentResponseHasNoSchema(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.DELETE("/payments", (&testfixtures.PaymentHandler{}).Delete)

	options := &Options{}
	WithConfig(NewDevelopmentConfig())(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)

	document, err := generator.GenerateSpec()
	require.NoError(t, err)

	operation := document.Paths["/payments"].Delete
	require.NotNil(t, operation)
	response, exists := operation.Responses["204"]
	require.True(t, exists)
	assert.Equal(t, "No Content", response.Description)
	assert.Empty(t, response.Content)
	assert.NotContains(t, operation.Responses, "200")
	assert.NotContains(t, document.Components.Schemas, "DELETE_paymentsresponse")
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

func TestFormQueryAndContentTypesAreDocumented(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	pages := &testfixtures.PageHandler{}
	engine.POST("/login", pages.Login)
	engine.POST("/logout", pages.Logout)
	engine.POST("/bound-upload", pages.BoundUpload)
	engine.POST("/raw-body", pages.RawBody)
	engine.POST("/dynamic-form", pages.DynamicPostForm)
	engine.GET("/report", pages.Report)

	options := &Options{}
	WithConfig(NewDevelopmentConfig())(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)
	generator.GetOverrideManager().Override("GET", "/report", RouteMetadata{
		Parameters: []spec.Parameter{{Name: "refresh", In: "query", Description: "Force refresh", Schema: spec.Schema{Type: "boolean"}}},
	})

	document, err := generator.GenerateSpec()
	require.NoError(t, err)

	login := document.Paths["/login"].Post
	require.NotNil(t, login)
	require.NotNil(t, login.RequestBody)
	assert.Contains(t, login.RequestBody.Content, "application/x-www-form-urlencoded")
	assert.NotContains(t, login.RequestBody.Content, "application/json")
	assert.Contains(t, document.Components.Schemas["POST_loginrequest"].Properties, "username")
	require.Len(t, login.Parameters, 1)
	assert.Equal(t, "next", login.Parameters[0].Name)
	assert.Equal(t, "query", login.Parameters[0].In)
	assert.Contains(t, login.Responses["200"].Content, "text/html")

	assert.Nil(t, document.Paths["/logout"].Post.RequestBody)

	boundUpload := document.Paths["/bound-upload"].Post
	require.NotNil(t, boundUpload.RequestBody)
	assert.Contains(t, boundUpload.RequestBody.Content, "multipart/form-data")
	assert.Contains(t, document.Components.Schemas["POST_bound-uploadrequest"].Properties, "name")
	assert.NotNil(t, document.Paths["/raw-body"].Post.RequestBody)
	assert.NotNil(t, document.Paths["/dynamic-form"].Post.RequestBody)

	report := document.Paths["/report"].Get
	require.NotNil(t, report)
	assert.Contains(t, report.Responses["200"].Content, "text/markdown")
	require.Len(t, report.Parameters, 1)
	assert.Equal(t, "Force refresh", report.Parameters[0].Description)
	assert.Equal(t, "boolean", report.Parameters[0].Schema.Type)
}

func TestSuccessfulNonJSONResponseWinsOverErrorJSON(t *testing.T) {
	schema := integration.NewGinHandlerAnalyzer().AnalyzeHandler((&testfixtures.PageHandler{}).MixedResponse)
	assert.Equal(t, 201, schema.ResponseStatus)
	assert.Equal(t, "text/html", schema.ResponseContentType)
	assert.Equal(t, "string", schema.ResponseSchema.Type)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/mixed-response", (&testfixtures.PageHandler{}).MixedResponse)
	options := &Options{}
	WithConfig(NewDevelopmentConfig())(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)
	document, err := generator.GenerateSpec()
	require.NoError(t, err)
	response := document.Paths["/mixed-response"].Post.Responses["201"]
	require.Contains(t, response.Content, "text/html")
	assert.Equal(t, "string", document.Components.Schemas["POST_mixed-responseresponse"].Type)
}

func TestMetadataContentTypeOverrides(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/signin", (&testfixtures.PageHandler{}).Login)
	engine.GET("/export", func(c *gin.Context) {
		_, _ = c.Writer.Write([]byte("csv"))
	})

	options := &Options{}
	WithConfig(NewDevelopmentConfig())(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)
	generator.GetOverrideManager().Override("POST", "/signin", RouteMetadata{
		RequestContentType:  "application/xml",
		ResponseContentType: "application/xml",
	})
	generator.GetOverrideManager().Override("GET", "/export", RouteMetadata{
		ResponseContentType: "text/csv",
	})

	document, err := generator.GenerateSpec()
	require.NoError(t, err)
	operation := document.Paths["/signin"].Post
	require.NotNil(t, operation.RequestBody)
	assert.Contains(t, operation.RequestBody.Content, "application/xml")
	assert.Contains(t, operation.Responses["200"].Content, "application/xml")
	assert.Contains(t, document.Paths["/export"].Get.Responses["200"].Content, "text/csv")
}

func TestSummariesTagsAndTagDescriptions(t *testing.T) {
	pathParser := parser.NewPathParser()

	list := pathParser.ParseRoute("GET", "/admin/jobs")
	item := pathParser.ParseRoute("GET", "/admin/jobs/:id")
	assert.Equal(t, "admin-jobs", list.Tag)
	assert.Equal(t, "admin-jobs", item.Tag)
	assert.Equal(t, "Get Admin Jobs", list.Summary)
	assert.Equal(t, "Get Admin Jobs By Id", item.Summary)
	assert.Contains(t, item.Description, "GET /admin/jobs/:id")
	assert.Equal(t, "admin", pathParser.ParseRoute("GET", "/admin").Tag)
	assert.Equal(t, "healthz", pathParser.ParseRoute("GET", "/healthz").Tag)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin/jobs", func(c *gin.Context) {})
	options := &Options{}
	WithConfig(NewDevelopmentConfig())(options)
	WithLogger(&testLogger{})(options)
	generator, err := NewGenerator(engine, integration.NewGinServerAdapter(engine), options)
	require.NoError(t, err)
	generator.GetOverrideManager().OverrideTagDescription("admin-jobs", "Background job administration")

	document, err := generator.GenerateSpec()
	require.NoError(t, err)
	require.Len(t, document.Tags, 1)
	assert.Equal(t, "Background job administration", document.Tags[0].Description)
}

func TestTagsOmitEmptyExternalDocs(t *testing.T) {
	data, err := json.Marshal(spec.Tag{Name: "payments"})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "externalDocs")
}
