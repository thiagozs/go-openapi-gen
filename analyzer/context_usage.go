package analyzer

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"github.com/thiagozs/go-openapi-gen/spec"
)

const (
	// ContentTypeJSON is the default media type of requests and responses.
	ContentTypeJSON = "application/json"
	// ContentTypeForm is the media type of url-encoded HTML form posts.
	ContentTypeForm = "application/x-www-form-urlencoded"
	// ContentTypeMultipart is the media type of file uploads.
	ContentTypeMultipart = "multipart/form-data"
)

// ContextUsage describes how a handler reads query/form input and which
// non-JSON content it writes, as seen on the framework context (gin.Context
// or hertz app.RequestContext).
type ContextUsage struct {
	QueryParams         []spec.Parameter
	FormFields          []string
	FileFields          []string
	ResponseContentType string
	ResponseStatus      int
	DelegatesContext    bool
	ReadsRequestBody    bool
	responseScore       int
}

var (
	queryMethods = map[string]bool{"Query": true, "DefaultQuery": true, "GetQuery": true}
	formMethods  = map[string]bool{"PostForm": true, "DefaultPostForm": true, "GetPostForm": true}
)

// InspectContextUsage walks a handler body and records the query parameters,
// form fields, uploaded files and non-JSON response content types it uses.
func InspectContextUsage(body *ast.BlockStmt, info *types.Info) ContextUsage {
	var usage ContextUsage
	if body == nil || info == nil {
		return usage
	}
	seenQuery := map[string]bool{}
	seenForm := map[string]bool{}
	seenFile := map[string]bool{}
	usage.responseScore = -1

	ast.Inspect(body, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Body" {
			request, ok := selector.X.(*ast.SelectorExpr)
			if ok && request.Sel.Name == "Request" && isFrameworkContext(info.TypeOf(request.X)) {
				usage.ReadsRequestBody = true
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if isSelector && isFrameworkContext(info.TypeOf(selector.X)) {
			inspectContextCall(selector.Sel.Name, call, info, &usage, seenQuery, seenForm, seenFile)
			return true
		}
		for _, argument := range call.Args {
			if isFrameworkContext(info.TypeOf(argument)) {
				usage.DelegatesContext = true
			}
		}
		return true
	})
	return usage
}

func inspectContextCall(method string, call *ast.CallExpr, info *types.Info, usage *ContextUsage, seenQuery, seenForm, seenFile map[string]bool) {
	switch {
	case queryMethods[method]:
		name, ok := constantString(call, 0, info)
		if !ok || seenQuery[name] {
			return
		}
		seenQuery[name] = true
		param := spec.Parameter{Name: name, In: "query", Schema: spec.Schema{Type: "string"}}
		if method == "DefaultQuery" {
			if value, ok := constantString(call, 1, info); ok {
				param.Schema.Default = value
			}
		}
		usage.QueryParams = append(usage.QueryParams, param)
	case formMethods[method]:
		usage.ReadsRequestBody = true
		if name, ok := constantString(call, 0, info); ok && !seenForm[name] {
			seenForm[name] = true
			usage.FormFields = append(usage.FormFields, name)
		}
	case method == "FormFile":
		usage.ReadsRequestBody = true
		if name, ok := constantString(call, 0, info); ok && !seenFile[name] {
			seenFile[name] = true
			usage.FileFields = append(usage.FileFields, name)
		}
	case method == "GetRawData":
		usage.ReadsRequestBody = true
	case method == "String":
		usage.setResponse(call, "text/plain", info)
	case method == "HTML":
		usage.setResponse(call, "text/html", info)
	case method == "XML":
		usage.setResponse(call, "application/xml", info)
	case method == "Data" && len(call.Args) >= 2:
		if contentType, ok := constantString(call, 1, info); ok {
			usage.setResponse(call, contentType, info)
		}
	}
}

func (u *ContextUsage) setResponse(call *ast.CallExpr, contentType string, info *types.Info) {
	if len(call.Args) == 0 {
		return
	}
	status, known := constantStatusCode(call.Args[0], info)
	score := 100
	if known {
		if status < 200 || status >= 300 {
			return
		}
		score = 200
	}
	if score <= u.responseScore {
		return
	}
	u.responseScore = score
	u.ResponseContentType = contentType
	u.ResponseStatus = status
}

func constantStatusCode(expr ast.Expr, info *types.Info) (int, bool) {
	value := info.Types[expr].Value
	if value == nil {
		return 0, false
	}
	status, exact := constant.Int64Val(value)
	return int(status), exact
}

// ApplyContextUsage merges the detected context usage into the schema.
// hasSuccessfulJSONResponse tells whether a successful JSON response was
// already detected, in which case non-JSON writes are not selected.
func (s *HandlerSchema) ApplyContextUsage(usage ContextUsage, hasSuccessfulJSONResponse bool) {
	jsonResponseScore := -1
	if hasSuccessfulJSONResponse {
		jsonResponseScore = 200
	}
	s.applyContextUsage(usage, jsonResponseScore, schemaSet(s.ResponseSchema))
}

// ApplyContextUsageWithResponseScore selects a non-JSON response only when it
// outranks the best JSON response candidate.
func (s *HandlerSchema) ApplyContextUsageWithResponseScore(usage ContextUsage, jsonResponseScore int, hasJSONSchema bool) {
	s.applyContextUsage(usage, jsonResponseScore, hasJSONSchema)
}

func (s *HandlerSchema) applyContextUsage(usage ContextUsage, jsonResponseScore int, hasJSONSchema bool) {
	s.Parameters = append(s.Parameters, usage.QueryParams...)

	if len(usage.FileFields) > 0 || len(usage.FormFields) > 0 {
		if !schemaSet(s.RequestSchema) {
			properties := make(map[string]spec.Schema, len(usage.FormFields)+len(usage.FileFields))
			for _, name := range usage.FormFields {
				properties[name] = spec.Schema{Type: "string"}
			}
			for _, name := range usage.FileFields {
				properties[name] = spec.Schema{Type: "string", Format: "binary"}
			}
			s.RequestSchema = spec.Schema{Type: "object", Properties: properties}
		}
		s.RequestContentType = ContentTypeForm
		if len(usage.FileFields) > 0 {
			s.RequestContentType = ContentTypeMultipart
		}
	}

	if !schemaSet(s.RequestSchema) {
		s.NoRequestBody = !usage.ReadsRequestBody && !usage.DelegatesContext
	}
	s.RequestBodyConsumed = usage.ReadsRequestBody || usage.DelegatesContext

	if usage.ResponseContentType != "" &&
		(usage.responseScore > jsonResponseScore || usage.responseScore == jsonResponseScore && !hasJSONSchema) {
		s.ResponseSchema = spec.Schema{Type: "string"}
		s.ResponseContentType = mediaTypeOnly(usage.ResponseContentType)
		s.ResponseStatus = usage.ResponseStatus
	}
}

// IsSet reports whether the analysis produced any information.
func (s HandlerSchema) IsSet() bool {
	return s.ResponseStatus != 0 || schemaSet(s.RequestSchema) || schemaSet(s.ResponseSchema) ||
		len(s.Parameters) > 0 || s.NoRequestBody || s.RequestBodyConsumed
}

func schemaSet(schema spec.Schema) bool {
	return schema.Type != "" || schema.Ref != "" || len(schema.AllOf) > 0 ||
		len(schema.OneOf) > 0 || len(schema.AnyOf) > 0
}

// mediaTypeOnly drops parameters such as "; charset=utf-8".
func mediaTypeOnly(contentType string) string {
	media, _, _ := strings.Cut(contentType, ";")
	return strings.TrimSpace(media)
}

func constantString(call *ast.CallExpr, index int, info *types.Info) (string, bool) {
	if index >= len(call.Args) {
		return "", false
	}
	value := info.Types[call.Args[index]].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

func isFrameworkContext(t types.Type) bool {
	if t == nil {
		return false
	}
	if pointer, ok := types.Unalias(t).(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	name, path := named.Obj().Name(), named.Obj().Pkg().Path()
	switch {
	case name == "Context" && strings.HasSuffix(path, "gin-gonic/gin"):
		return true
	case name == "RequestContext" && strings.Contains(path, "cloudwego/hertz"):
		return true
	}
	return false
}
