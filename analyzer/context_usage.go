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
	DelegatesContext    bool
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

	ast.Inspect(body, func(node ast.Node) bool {
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
		if name, ok := constantString(call, 0, info); ok && !seenForm[name] {
			seenForm[name] = true
			usage.FormFields = append(usage.FormFields, name)
		}
	case method == "FormFile":
		if name, ok := constantString(call, 0, info); ok && !seenFile[name] {
			seenFile[name] = true
			usage.FileFields = append(usage.FileFields, name)
		}
	case method == "String":
		usage.setResponseContentType("text/plain")
	case method == "HTML":
		usage.setResponseContentType("text/html")
	case method == "XML":
		usage.setResponseContentType("application/xml")
	case method == "Data" && len(call.Args) >= 2:
		if contentType, ok := constantString(call, 1, info); ok {
			usage.setResponseContentType(contentType)
		}
	}
}

func (u *ContextUsage) setResponseContentType(contentType string) {
	if u.ResponseContentType == "" {
		u.ResponseContentType = contentType
	}
}

// ApplyContextUsage merges the detected context usage into the schema.
// hasJSONResponse tells whether a JSON response body was already detected, in
// which case non-JSON writes (typically error pages) are ignored.
func (s *HandlerSchema) ApplyContextUsage(usage ContextUsage, hasJSONResponse bool) {
	s.Parameters = append(s.Parameters, usage.QueryParams...)

	if !schemaSet(s.RequestSchema) {
		switch {
		case len(usage.FileFields) > 0 || len(usage.FormFields) > 0:
			properties := make(map[string]spec.Schema, len(usage.FormFields)+len(usage.FileFields))
			for _, name := range usage.FormFields {
				properties[name] = spec.Schema{Type: "string"}
			}
			for _, name := range usage.FileFields {
				properties[name] = spec.Schema{Type: "string", Format: "binary"}
			}
			s.RequestSchema = spec.Schema{Type: "object", Properties: properties}
			s.RequestContentType = ContentTypeForm
			if len(usage.FileFields) > 0 {
				s.RequestContentType = ContentTypeMultipart
			}
		case !usage.DelegatesContext:
			s.NoRequestBody = true
		}
	}

	if !hasJSONResponse && !schemaSet(s.ResponseSchema) && usage.ResponseContentType != "" {
		s.ResponseSchema = spec.Schema{Type: "string"}
		s.ResponseContentType = mediaTypeOnly(usage.ResponseContentType)
	}
}

// IsSet reports whether the analysis produced any information.
func (s HandlerSchema) IsSet() bool {
	return s.ResponseStatus != 0 || schemaSet(s.RequestSchema) || schemaSet(s.ResponseSchema) ||
		len(s.Parameters) > 0 || s.NoRequestBody
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
