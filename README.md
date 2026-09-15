# OpenAPI Generator for Go Web Frameworks

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.27-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A powerful, framework-agnostic OpenAPI documentation generator for Go web applications. Automatically generates comprehensive OpenAPI 3.0.3 specifications from your route definitions with intelligent AST analysis, Docker support, and flexible customization options. Works seamlessly as a library in any Go application with production-ready fallback mechanisms.

## ✨ Features

- **🚀 Zero Configuration**: Works out of the box with sensible defaults
- **🧠 Intelligent Generation**: AST analysis of route paths and handlers for accurate schemas
- **🎨 Flexible Customization**: Override any aspect of the generated documentation
- **📝 Multiple Frameworks**: Extensible architecture supports any Go web framework
- **⚡ High Performance**: Zero runtime cost, documentation generated at startup
- **🔧 Options Pattern**: Clean, extensible API with functional options
- **📊 Generic Logging**: Integrate with any logging framework
- **🐳 Docker Ready**: Explicit per-route schemas when source files are unavailable
- **📦 Library Support**: Works seamlessly as external library in any Go application
- **🔍 AST Analysis**: Automatic request/response schema generation from handler code
- **🏗️ Compile-time Schemas**: Generate Go schema registrations before production builds

## 🚀 Quick Start

### Installation

```bash
go get github.com/thiagozs/go-openapi-gen
```

### Basic Usage

#### CloudWeGo Hertz

```go
package main

import (
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/thiagozs/go-openapi-gen"
    "github.com/thiagozs/go-openapi-gen/integration"
)

func main() {
    h := server.Default()
    
    // Add your routes here
    h.GET("/api/v1/users", getUsersHandler)
    h.POST("/api/v1/users", createUserHandler)
    
    // Enable OpenAPI documentation with proper library usage
    err := openapi.EnableDocs(h, integration.NewHertzServerAdapter(h))
    if err != nil {
        panic(err)
    }
    
    h.Spin()
}
```

#### Gin

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/thiagozs/go-openapi-gen"
    "github.com/thiagozs/go-openapi-gen/integration"
)

func main() {
    r := gin.Default()
    
    // Add your routes here
    r.GET("/api/v1/users", getUsersHandler)
    r.POST("/api/v1/users", createUserHandler)
    
    // Enable OpenAPI documentation with proper library usage
    err := openapi.EnableDocs(r, integration.NewGinServerAdapter(r))
    if err != nil {
        panic(err)
    }
    
    r.Run()
}
```

**That's it!** Your API documentation is now available at:

- **Swagger UI**: `http://localhost:8080/docs`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

## 🐳 Docker & Production Usage

### Development vs Production

```go
// Development Mode (with AST analysis)
func setupDevelopment() error {
    h := server.Default()
    return openapi.EnableDocs(h, integration.NewHertzServerAdapter(h))
}

// Production Mode (with static schemas)
func setupProduction() error {
    h := server.Default()
    return openapi.EnableDocs(h, integration.NewHertzServerAdapter(h),
        openapi.WithSchemaDir("./schemas"),
    )
}
```

### Docker Build with Generated Schemas

Run code generation in the build stage; the schemas are compiled into the binary:

```dockerfile
# Build stage
FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download

# Generate zz_openapi_gen.go files
RUN go generate ./...

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .

# Production stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Only the binary is needed
COPY --from=builder /app/myapp .

CMD ["./myapp"]
```

## 🏗️ Production Schema Generation

For source-free production images, generate Go code containing the schemas before
building the application. Add one directive to a file in each package containing
Gin handlers:

```go
//go:generate go run github.com/thiagozs/go-openapi-gen/cmd/openapi-gen
package handlers
```

The generator finds `ShouldBind*`/`Bind*` request types and structured `JSON`
response types, then writes `zz_openapi_gen.go`. Its `init` function registers
the schemas before the OpenAPI `Generator` is created, so no source or external
schema files are required in the final container.

### Generating Schema Code

Generate schema code using the CLI tool:

```bash
# Run every directive in the project
go generate ./...

# Or generate one handler package directly
go run github.com/thiagozs/go-openapi-gen/cmd/openapi-gen ./internal/handlers

# Install the tool globally
go install github.com/thiagozs/go-openapi-gen/cmd/openapi-gen@latest
openapi-gen ./internal/handlers
```

This creates `zz_openapi_gen.go` in the handler package.

### CLI Tool Features

The `cmd/openapi-gen/` tool provides advanced schema generation capabilities:

- **Automatic struct analysis** - Parses request and response structs using `go/types`
- **JSON tag support** - Uses JSON tag names instead of Go variable names  
- **Self-contained binary** - Generated schemas are compiled into the application
- **Type-aware generation** - Handles basic types, arrays, maps, pointers, and custom types
- **Validation tags** - Supports `validate` and Gin `binding` constraints

### CLI Options

```bash
openapi-gen [options] [package-directory]

Options:
  -output string     Generated Go file (default "zz_openapi_gen.go")
  -verbose           Enable verbose output
  -handler string    Handler name (auto-detected if not provided)
  -framework string  Handler framework (currently "gin")
```

### Example Usage

```bash
# Generate all handlers in a package
openapi-gen -verbose ./internal/handlers

# Generate only one named handler
openapi-gen -handler Login ./internal/handlers

# Generate all schemas in project
go generate ./...
```

### Generated Code

Each handler package gets a Go file that registers its schemas during package
initialization:

```go
// Code generated by openapi-gen. DO NOT EDIT.
func init() {
    openapianalyzer.RegisterGeneratedHandlerSchema(
        "example.com/myapp/handlers.AuthHandler.Login",
        /* generated schema */,
    )
}
```

### Using Generated Schemas in Production

No runtime configuration is required. Import and register the handler package as
usual, then build after running `go generate`:

```go
func main() {
    r := gin.Default()
    paymentHandler := handlers.NewPaymentHandler()
    
    // Add your routes here
    r.POST("/payments", paymentHandler.Create)
    
    // Generated schemas were registered by the handlers package init function.
    err := openapi.EnableDocs(r, integration.NewGinServerAdapter(r))
    if err != nil {
        panic(err)
    }
    
    r.Run(":8080")
}
```

## 🎯 Advanced Usage

### Custom Configuration

```go
// Create custom configuration
cfg := openapi.NewConfig()
cfg.Title = "My Awesome API"
cfg.Description = "A comprehensive API for my application"
cfg.Version = "2.0.0"
cfg.Contact.Name = "API Team"
cfg.Contact.Email = "api@example.com"

// Enable with custom config
err := openapi.EnableDocs(framework, integration.NewHertzServerAdapter(framework),
    openapi.WithConfig(cfg),
)
```

### Environment-Based Configuration

```go
import "os"

func getOpenAPIConfig() *openapi.Config {
    cfg := openapi.NewConfig()
    
    if os.Getenv("ENV") == "production" {
        cfg.SchemaDir = "./schemas"
    }
    
    return cfg
}

// Use environment-based config
err := openapi.EnableDocs(h, integration.NewHertzServerAdapter(h),
    openapi.WithConfig(getOpenAPIConfig()),
)
```

### Custom Logging

```go
// With slog (recommended)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithSlogLogger(logger),
)

// With any custom logger
type MyLogger struct{}
func (l *MyLogger) Info(msg string, args ...any) { /* implementation */ }
func (l *MyLogger) Warn(msg string, args ...any) { /* implementation */ }
func (l *MyLogger) Error(msg string, args ...any) { /* implementation */ }
func (l *MyLogger) Debug(msg string, args ...any) { /* implementation */ }

err := openapi.EnableDocs(framework, httpServer,
    openapi.WithLogger(&MyLogger{}),
)
```

### Route Customization

```go
import "github.com/thiagozs/go-openapi-gen/example"

err := openapi.EnableDocs(framework, httpServer,
    openapi.WithCustomizer(example.CustomizeAuthentication),
    openapi.WithCustomizer(example.CustomizeMFA),
    openapi.WithCustomizer(func(generator *openapi.Generator) error {
        om := generator.GetOverrideManager()
        om.Override("GET", "/api/v1/users", openapi.RouteMetadata{
            Tags:        "users",
            Summary:     "List Users",
            Description: "Retrieve a paginated list of all users",
        })
        return nil
    }),
)
```

### Custom Framework Integration

```go
// Implement the RouteDiscoverer interface for your framework
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
)
```

## 🏗️ Architecture

### Three Levels of Customization

1. **🤖 Algorithmic**: Intelligent generation from route paths
2. **📋 Preset**: Common patterns applied automatically  
3. **✏️ Custom**: User-defined overrides for specific needs

### Example Generated Documentation

| Route | Generated | Enhanced | Custom |
| ------- | ----------- | ---------- | --------- |
| `POST /api/v1/auth/login` | `Create Auth Login` | `User Authentication` | `Authenticate user with email and password. Returns JWT tokens.` |
| `GET /api/v1/users/:id` | `Get Users Id` | `Get User Details` | `Retrieve user information by unique identifier` |

## 🔧 Options API

All configuration is done through functional options:

```go
openapi.EnableDocs(framework, integration.NewHertzServerAdapter(framework),
    // Configuration Options
    openapi.WithConfig(cfg),                    // Custom configuration
    openapi.WithSchemaDir("./schemas"),        // Static schema files directory
    
    // Logging & Discovery
    openapi.WithSlogLogger(logger),            // slog logger (convenience)
    openapi.WithLogger(customLogger),          // Any logger interface
    openapi.WithRouteDiscoverer(discoverer),   // Custom framework integration
    openapi.WithCustomizer(customizeFunc),     // Route customizations
)
```

## 🌐 Framework Support

### Currently Supported

- ✅ **CloudWeGo Hertz** - Full auto-detection support
- ✅ **Gin** - Full auto-detection support

### Coming Soon

- 🔄 **Echo** - Interface ready, implementation planned  
- 🔄 **Fiber** - Interface ready, implementation planned
- 🔄 **Chi** - Interface ready, implementation planned

*Framework integration is designed to be pluggable. Contributions welcome!*

## 📚 Documentation

- **[Examples](./example/README.md)**: Comprehensive usage examples
- **[Customization Guide](./example/README.md#customization-types)**: Learn how to customize documentation
- **[Framework Integration](./example/README.md#custom-framework-integration)**: Add support for your framework

## 🚨 Troubleshooting

### Common Issues

#### Generic schemas in production

When source files aren't available, the generator uses fallback schemas.

**Solutions**:

1. **Use explicit route schema overrides** (currently supported):

   ```go
   schema := spec.Schema{Type: "object", Properties: map[string]spec.Schema{
       "amount": {Type: "number"},
   }}
   generator.GetOverrideManager().Override("POST", "/payments", openapi.RouteMetadata{
       RequestSchema: &schema,
   })
   ```

2. **Generate schemas into the binary** before the application build:

   ```dockerfile
   # Build stage
   FROM golang:1.27-alpine AS builder
   WORKDIR /app
   COPY . .
   RUN go mod download
   
   # Run //go:generate directives that create zz_openapi_gen.go
   RUN go generate ./...
   
   # Build the application
   RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .
   
   # Production stage
   FROM alpine:latest
   COPY --from=builder /app/myapp .
   CMD ["./myapp"]
   ```

#### Import path issues when using as library

Make sure to use the correct import paths:

```go
import (
    "github.com/thiagozs/go-openapi-gen"
    "github.com/thiagozs/go-openapi-gen/integration"
)

// Correct usage
openapi.EnableDocs(h, integration.NewHertzServerAdapter(h))
```

### Environment Variables

Control behavior with environment variables:

```bash
OPENAPI_SCHEMA_DIR=./schemas    # Set schema files directory
```

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines for details.

### TODO Roadmap

- [x] **Docker and Production Support** - Explicit route schema overrides
- [x] **Library Usage Improvements** - Cross-package AST analysis and configuration options  
- [x] **AST-based Schema Generation** - Automatic request/response schema extraction
- [x] **go:generate Schema Generation** - Compile-time schema generation CLI for Gin
- [x] **Gin Framework Support** - Complete integration with Gin framework
- [ ] Additional framework integrations (Echo, Fiber, Chi)
- [ ] Plugin system for custom analyzers
- [ ] OpenAPI 3.1 support
- [ ] Performance optimizations for large APIs
- [ ] Enhanced validation tag support

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built with ❤️ using:

- [CloudWeGo Hertz](https://github.com/cloudwego/hertz) - High-performance Go HTTP framework
- [Swagger UI](https://swagger.io/tools/swagger-ui/) - Interactive API documentation

---

**Made with ❤️ for the Go community**
