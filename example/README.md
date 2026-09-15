# OpenAPI Customization Examples

This directory contains examples of how to customize the automatically generated OpenAPI documentation for your auth service.

## Overview

The OpenAPI generator provides three levels of customization:

1. **Algorithm**: Pure algorithmic generation from route paths
2. **Presets**: Common patterns applied automatically  
3. **Custom**: User-defined overrides for specific needs

## Basic Usage

### Simple Integration (Recommended)
```go
// In your main.go
import "github.com/thiagozs/go-openapi-gen"

// Minimal setup with defaults
err := openapi.EnableDocs(framework, httpServer)
if err != nil {
    // handle error
}
```

**Result**: Access documentation at:
- **Swagger UI**: `http://localhost:8080/docs`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

### Custom Configuration
```go
import (
    "log/slog"
    "os"
    "github.com/thiagozs/go-openapi-gen"
)

// Custom config and logger
cfg := openapi.NewConfig()
cfg.Title = "Your API"
cfg.Description = "Your API description" 
cfg.Version = "1.0.0"
cfg.SchemaDir = "./schemas"  // Directory for static schema files

// Option 1: Use slog (recommended, convenience function)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
    openapi.WithSlogLogger(logger),  // Convenience function for slog
)

// Option 2: Use any custom logger implementing Logger interface
type MyLogger struct{}
func (l *MyLogger) Info(msg string, args ...any) { /* your implementation */ }
func (l *MyLogger) Warn(msg string, args ...any) { /* your implementation */ }
func (l *MyLogger) Error(msg string, args ...any) { /* your implementation */ }
func (l *MyLogger) Debug(msg string, args ...any) { /* your implementation */ }

err = openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
    openapi.WithLogger(&MyLogger{}),  // Any logger implementing the interface
)

if err != nil {
    // handle error
}
```

### Advanced Integration with Customization
```go
import (
    "github.com/thiagozs/go-openapi-gen"
    "github.com/thiagozs/go-openapi-gen/example"
)

// Multiple customizations with options pattern
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
    openapi.WithLogger(logger),
    openapi.WithCustomizer(example.CustomizeAuthentication),
    openapi.WithCustomizer(example.CustomizeMFA),
    openapi.WithCustomizer(example.CustomizeOAuth),
)
if err != nil {
    // handle error
}
```

### Custom Framework Integration
```go
import "github.com/thiagozs/go-openapi-gen"

// Implement custom route discoverer for your framework
type MyFrameworkDiscoverer struct {
    framework *MyFramework
}

func (d *MyFrameworkDiscoverer) DiscoverRoutes() ([]spec.RouteInfo, error) {
    // Your custom route discovery logic
    return routes, nil
}

func (d *MyFrameworkDiscoverer) GetFrameworkName() string {
    return "MyFramework"
}

// Use with custom discoverer
discoverer := &MyFrameworkDiscoverer{framework: myFramework}
err := openapi.EnableDocs(myFramework, httpServer,
    openapi.WithRouteDiscoverer(discoverer),
    openapi.WithCustomizer(example.CustomizeAuthentication),
)
```

## Schema Generation with go:generate

The CLI generates Go code containing Gin handler schemas so production images do
not need source or external JSON files.

### Adding go:generate Annotations

Add one directive to each package containing Gin handlers:

```go
//go:generate go run github.com/thiagozs/go-openapi-gen/cmd/openapi-gen
package handlers
```

### Generating Schema Code

Generate schema code using the CLI tool:

```bash
go generate ./...

# Or invoke the generator for one package
go run github.com/thiagozs/go-openapi-gen/cmd/openapi-gen ./internal/handlers
```

This creates `zz_openapi_gen.go`, which is compiled into the application.

### Using Generated Schemas in Production

No additional runtime configuration is necessary; register routes and enable the
documentation normally after running `go generate`.

```go
func main() {
    r := gin.Default()
    paymentHandler := handlers.NewPaymentHandler()
    
    // Add your routes here
    r.POST("/payments", paymentHandler.Create)
    
    // Generated schemas are registered by the handlers package init function.
    err := openapi.EnableDocs(r, integration.NewGinServerAdapter(r))
    if err != nil {
        panic(err)
    }
    
    r.Run(":8080")
}
```

## What Gets Generated

### Pure Algorithm Results
The system automatically generates documentation from your routes:

| Route | Generated Tag | Generated Summary | Generated Description |
|-------|---------------|-------------------|----------------------|
| `POST /api/v1/auth/login` | `auth` | `Create Auth Login` | `Create Auth Login operation` |
| `GET /api/v1/oauth/providers` | `oauth` | `Get Oauth Providers` | `Get Oauth Providers operation` |
| `POST /api/v1/user/mfa/setup` | `user` | `Create User Mfa Setup` | `Create User Mfa Setup operation` |
| `GET /health` | `health` | `Get Health` | `Get Health operation` |

### With Presets Applied
Presets improve the algorithmic results:

| Route | Enhanced Tags | Enhanced Summary | Enhanced Description |
|-------|---------------|------------------|----------------------|
| `POST /api/v1/auth/login` | `authentication` | `User Authentication` | `Authenticate user and return authentication tokens` |
| `GET /health` | `system` | `Service Health Check` | `Get comprehensive service health status...` |

### With Custom Overrides
Your custom functions can provide perfect documentation:

| Route | Custom Tags | Custom Summary | Custom Description |
|-------|-------------|----------------|-------------------|
| `POST /api/v1/auth/login` | `authentication` | `User Authentication` | `Authenticate user with email and password. Returns JWT access token and refresh token for session management.` |

## Available Customization Functions

See the example files in this directory:

- **`authentication.go`** - Authentication endpoints customization
- **`mfa.go`** - Multi-factor authentication customization  
- **`oauth.go`** - OAuth provider customization
- **`patterns.go`** - Pattern-based customization examples

## Customization Types

### 1. Exact Path Override
```go
om.Override("POST", "/api/v1/auth/login", openapi.RouteMetadata{
    Tags:        "authentication",
    Summary:     "User Authentication",
    Description: "Authenticate user with email and password",
})
```

### 2. Pattern-Based Override
```go
om.OverridePattern("POST */login", openapi.RouteMetadata{
    Summary:     "Login Operation",
    Description: "Authenticate user via login endpoint",
})
```

### 3. Tag-Level Override
```go
om.OverrideTags("auth", "authentication")
```

## Performance Notes

- **Zero Runtime Cost**: Documentation is generated once on startup
- **Development Mode**: Spec regenerated on each request for live updates

## Logging Options

The OpenAPI generator supports flexible logging through a generic Logger interface, allowing integration with any logging framework.

### Built-in Logger Support

```go
// Default behavior - uses slog.Default()
err := openapi.EnableDocs(framework, httpServer)

// Custom slog logger (convenience function)
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithSlogLogger(logger),
)

// No-op logger (silent operation)
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithLogger(&openapi.NoOpLogger{}),
)
```

### Custom Logger Integration

Implement the Logger interface for any logging framework:

```go
// Example with Logrus (hypothetical adapter)
type LogrusAdapter struct {
    logger *logrus.Logger
}

func (l *LogrusAdapter) Info(msg string, args ...any) {
    l.logger.WithFields(argsToFields(args)).Info(msg)
}
// ... implement Warn, Error, Debug methods

// Usage
logrusLogger := logrus.New()
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithLogger(&LogrusAdapter{logger: logrusLogger}),
)
```

The Logger interface is simple and easy to implement:

```go
type Logger interface {
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    Debug(msg string, args ...any)
}
```

## Framework Support

Currently supported:
- ✅ **CloudWeGo Hertz** (auto-detection via framework interface{})

Framework detection is automatic - just pass your framework instance:
```go
// Works with CloudWeGo Hertz out of the box
err := openapi.EnableDocs(hertzFramework, httpServer)

// For custom frameworks, provide your own discoverer
err := openapi.EnableDocs(customFramework, httpServer,
    openapi.WithRouteDiscoverer(myCustomDiscoverer),
)
```

## Troubleshooting

### Common Issues

1. **No routes found**: Ensure `EnableDocs` is called after route registration
2. **Pattern not matching**: Use debug logging to check regex patterns
3. **Missing schemas**: DTO types need to be exported structs
4. **Generic schemas in production**: When source files aren't available, the generator uses fallback schemas

**Solutions for production schemas**:
1. **Generate schemas into the binary**:
   ```go
   //go:generate go run github.com/thiagozs/go-openapi-gen/cmd/openapi-gen
   package handlers
   ```

2. **Run generation before the Docker build**:
   ```dockerfile
   # Build stage
   FROM golang:1.27-alpine AS builder
   WORKDIR /app
   COPY . .
   RUN go mod download
   RUN go generate ./...
   RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .
   
   # Production stage
   FROM alpine:latest
   COPY --from=builder /app/myapp .
   CMD ["./myapp"]
   ```

### Debug Mode
```go
// Enable debug logging
om := generator.GetOverrideManager()
overrides := om.ListOverrides()       // See active overrides
stats := om.GetOverrideStats()       // Get override statistics
```

### Testing
```bash
# Test your customizations
go test ./...

# Check generated spec
curl http://localhost:8080/openapi.json | jq .

# Test with different configurations
go run main.go  # with defaults
go run examples/custom-config/main.go  # with custom config
go run examples/custom-framework/main.go  # with custom discoverer
```
