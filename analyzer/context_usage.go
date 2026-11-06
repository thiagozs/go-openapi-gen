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
	// RedirectStatus is the 3xx status of a c.Redirect call, directly in the
	// handler or in a same-package helper that receives the context.
	RedirectStatus int
	responseScore  int
}

// redirectScore ranks a redirect above error-only handlers (non-2xx) and above
// writes with a status only known at run time (usually the error path of a
// post/redirect/get handler), but below any constant 2xx response.
const redirectScore = 100

const maxHelperDepth = 3

var (
	queryMethods = map[string]bool{"Query": true, "DefaultQuery": true, "GetQuery": true}
	formMethods  = map[string]bool{"PostForm": true, "DefaultPostForm": true, "GetPostForm": true}
)

// InspectContextUsage walks a handler body and records the query parameters,
// form fields, uploaded files and non-JSON response content types it uses.
func InspectContextUsage(body *ast.BlockStmt, info *types.Info) ContextUsage {
	return InspectContextUsageIn(body, info, nil)
}

// InspectContextUsageIn is InspectContextUsage that also follows same-package
// helpers receiving the context (up to maxHelperDepth levels), so form, query,
// redirect and content-type usage hidden behind helpers is still detected.
func InspectContextUsageIn(body *ast.BlockStmt, info *types.Info, files []*ast.File) ContextUsage {
	w := &usageWalker{
		info:      info,
		decls:     funcDecls(files, info),
		visited:   map[*types.Func]bool{},
		seenQuery: map[string]bool{},
		seenForm:  map[string]bool{},
		seenFile:  map[string]bool{},
	}
	w.usage.responseScore = -1
	if body == nil || info == nil {
		return w.usage
	}
	w.walk(body, 0)
	return w.usage
}

type usageWalker struct {
	info      *types.Info
	decls     map[*types.Func]*ast.FuncDecl
	visited   map[*types.Func]bool
	usage     ContextUsage
	seenQuery map[string]bool
	seenForm  map[string]bool
	seenFile  map[string]bool
}

func (w *usageWalker) walk(body *ast.BlockStmt, depth int) {
	info := w.info
	ast.Inspect(body, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Body" {
			request, ok := selector.X.(*ast.SelectorExpr)
			if ok && request.Sel.Name == "Request" && isFrameworkContext(info.TypeOf(request.X)) {
				w.usage.ReadsRequestBody = true
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if isSelector && isFrameworkContext(info.TypeOf(selector.X)) {
			inspectContextCall(selector.Sel.Name, call, info, &w.usage, w.seenQuery, w.seenForm, w.seenFile)
			return true
		}
		if isSelector && selector.Sel.Name == "Set" {
			if contentType, ok := writerContentType(call, selector, info); ok {
				w.usage.setContentType(contentType)
				return true
			}
		}
		delegates := false
		for _, argument := range call.Args {
			if isFrameworkContext(info.TypeOf(argument)) {
				delegates = true
			}
		}
		if !delegates {
			return true
		}
		w.usage.DelegatesContext = true
		if helper := w.helperBody(call); helper != nil && depth < maxHelperDepth {
			w.walk(helper, depth+1)
		}
		return true
	})
}

// helperBody returns the body of the same-package function called by call, once.
func (w *usageWalker) helperBody(call *ast.CallExpr) *ast.BlockStmt {
	var ident *ast.Ident
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return nil
	}
	fn, ok := w.info.Uses[ident].(*types.Func)
	if !ok || w.visited[fn] {
		return nil
	}
	decl := w.decls[fn]
	if decl == nil {
		return nil
	}
	w.visited[fn] = true
	return decl.Body
}

// writerContentType matches c.Writer.Header().Set("Content-Type", "<const>").
func writerContentType(call *ast.CallExpr, set *ast.SelectorExpr, info *types.Info) (string, bool) {
	header, ok := set.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return "", false
	}
	headerSel, ok := header.Fun.(*ast.SelectorExpr)
	if !ok || headerSel.Sel.Name != "Header" {
		return "", false
	}
	writer, ok := headerSel.X.(*ast.SelectorExpr)
	if !ok || writer.Sel.Name != "Writer" || !isFrameworkContext(info.TypeOf(writer.X)) {
		return "", false
	}
	return headerContentType(call, 0, info)
}

func headerContentType(call *ast.CallExpr, nameIndex int, info *types.Info) (string, bool) {
	name, ok := constantString(call, nameIndex, info)
	if !ok || !strings.EqualFold(name, "Content-Type") {
		return "", false
	}
	return constantString(call, nameIndex+1, info)
}

func funcDecls(files []*ast.File, info *types.Info) map[*types.Func]*ast.FuncDecl {
	decls := map[*types.Func]*ast.FuncDecl{}
	if info == nil {
		return decls
	}
	for _, file := range files {
		for _, declaration := range file.Decls {
			decl, ok := declaration.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			if fn, ok := info.Defs[decl.Name].(*types.Func); ok {
				decls[fn] = decl
			}
		}
	}
	return decls
}

// setContentType records a content type set through a header, whose status is
// not known statically.
func (u *ContextUsage) setContentType(contentType string) {
	if redirectScore <= u.responseScore {
		return
	}
	u.responseScore = redirectScore
	u.ResponseContentType = contentType
	u.ResponseStatus = 0
}

func redirectCode(method string, call *ast.CallExpr, info *types.Info) (int, bool) {
	if method != "Redirect" || len(call.Args) == 0 {
		return 0, false
	}
	code, known := constantStatusCode(call.Args[0], info)
	return code, known && code >= 300 && code < 400
}

func inspectContextCall(method string, call *ast.CallExpr, info *types.Info, usage *ContextUsage, seenQuery, seenForm, seenFile map[string]bool) {
	switch {
	case method == "Header":
		if contentType, ok := headerContentType(call, 0, info); ok {
			usage.setContentType(contentType)
		}
	case method == "Redirect":
		if code, ok := redirectCode(method, call, info); ok && usage.RedirectStatus == 0 {
			usage.RedirectStatus = code
		}
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

	if usage.RedirectStatus != 0 && s.ResponseStatus == 0 &&
		jsonResponseScore < redirectScore && usage.responseScore <= redirectScore {
		s.ResponseStatus = usage.RedirectStatus
		s.ResponseSchema = spec.Schema{}
		s.ResponseContentType = ""
		return
	}

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
