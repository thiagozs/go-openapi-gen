package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/thiagozs/go-openapi-gen/analyzer"
	"github.com/thiagozs/go-openapi-gen/spec"
	"golang.org/x/tools/go/packages"
)

const defaultOutput = "zz_openapi_gen.go"

type options struct {
	dir       string
	output    string
	handler   string
	framework string
	verbose   bool
}

type generatedHandler struct {
	name     string
	id       string
	request  spec.Schema
	response spec.Schema
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "openapi-gen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("openapi-gen", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	opts := options{}
	flags.StringVar(&opts.output, "output", defaultOutput, "generated Go file")
	flags.StringVar(&opts.handler, "handler", "", "generate only the named handler")
	flags.StringVar(&opts.framework, "framework", "gin", "handler framework (gin or hertz)")
	flags.BoolVar(&opts.verbose, "verbose", false, "print analyzed handlers")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("expected at most one package directory")
	}
	if opts.framework != "gin" && opts.framework != "hertz" {
		return fmt.Errorf("unsupported framework %q: expected gin or hertz", opts.framework)
	}
	opts.dir = "."
	if flags.NArg() == 1 {
		opts.dir = flags.Arg(0)
	}

	pkgName, handlers, err := analyzePackage(opts)
	if err != nil {
		return err
	}
	if len(handlers) == 0 {
		if opts.handler != "" {
			return fmt.Errorf("handler %q was not found or has no detectable schema", opts.handler)
		}
		return fmt.Errorf("no %s handlers with detectable request or response schemas found", opts.framework)
	}

	content, err := renderGeneratedFile(pkgName, handlers)
	if err != nil {
		return err
	}
	output := opts.output
	if !filepath.IsAbs(output) {
		output = filepath.Join(opts.dir, output)
	}
	if err := os.WriteFile(output, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	if opts.verbose {
		for _, handler := range handlers {
			fmt.Printf("generated schema for %s\n", handler.name)
		}
		fmt.Printf("wrote %s\n", output)
	}
	return nil
}

func analyzePackage(opts options) (string, []generatedHandler, error) {
	absDir, err := filepath.Abs(opts.dir)
	if err != nil {
		return "", nil, err
	}
	cfg := &packages.Config{
		Dir: absDir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedTypesSizes | packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return "", nil, fmt.Errorf("load package: %w", err)
	}
	if len(pkgs) != 1 {
		return "", nil, fmt.Errorf("expected one package, loaded %d", len(pkgs))
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return "", nil, fmt.Errorf("package contains errors: %v", pkg.Errors)
	}

	schemaGenerator := analyzer.NewSchemaGenerator()
	byID := make(map[string]generatedHandler)
	for _, file := range pkg.Syntax {
		for _, declaration := range file.Decls {
			decl, ok := declaration.(*ast.FuncDecl)
			if !ok || decl.Body == nil || (opts.handler != "" && decl.Name.Name != opts.handler) {
				continue
			}
			if !isFrameworkHandler(decl, pkg.TypesInfo, opts.framework) {
				continue
			}
			requestType, responseType := typesFromHandler(decl, pkg.TypesInfo, opts.framework)
			handler := generatedHandler{name: decl.Name.Name, id: declarationHandlerID(pkg.PkgPath, decl)}
			if requestType != nil {
				handler.request = schemaGenerator.GenerateSchemaFromGoType(requestType)
			}
			if responseType != nil {
				handler.response = schemaGenerator.GenerateSchemaFromGoType(responseType)
			}
			if !schemaSet(handler.request) && !schemaSet(handler.response) {
				continue
			}
			if _, exists := byID[handler.id]; exists {
				return "", nil, fmt.Errorf("duplicate handler identifier %q", handler.id)
			}
			byID[handler.id] = handler
		}
	}

	handlers := make([]generatedHandler, 0, len(byID))
	for _, handler := range byID {
		handlers = append(handlers, handler)
	}
	sort.Slice(handlers, func(i, j int) bool { return handlers[i].name < handlers[j].name })
	return pkg.Name, handlers, nil
}

func declarationHandlerID(packagePath string, decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return packagePath + "." + decl.Name.Name
	}
	receiver := decl.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return packagePath + "." + ident.Name + "." + decl.Name.Name
	}
	return packagePath + "." + decl.Name.Name
}

func isFrameworkHandler(decl *ast.FuncDecl, info *types.Info, framework string) bool {
	switch framework {
	case "gin":
		return isGinHandler(decl, info)
	case "hertz":
		return isHertzHandler(decl, info)
	default:
		return false
	}
}

func isGinHandler(decl *ast.FuncDecl, info *types.Info) bool {
	object := info.Defs[decl.Name]
	if object == nil {
		return false
	}
	signature, ok := object.Type().(*types.Signature)
	if !ok || signature.Params().Len() != 1 || signature.Results().Len() != 0 {
		return false
	}
	parameter := signature.Params().At(0).Type()
	pointer, ok := parameter.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := pointer.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == "github.com/gin-gonic/gin" && named.Obj().Name() == "Context"
}

func isHertzHandler(decl *ast.FuncDecl, info *types.Info) bool {
	object := info.Defs[decl.Name]
	if object == nil {
		return false
	}
	signature, ok := object.Type().(*types.Signature)
	if !ok || signature.Params().Len() != 2 || signature.Results().Len() != 0 {
		return false
	}
	contextType := signature.Params().At(0).Type()
	contextNamed, ok := contextType.(*types.Named)
	if !ok || contextNamed.Obj().Pkg() == nil ||
		contextNamed.Obj().Pkg().Path() != "context" || contextNamed.Obj().Name() != "Context" {
		return false
	}
	requestContext := signature.Params().At(1).Type()
	pointer, ok := requestContext.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := pointer.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == "github.com/cloudwego/hertz/pkg/app" &&
		named.Obj().Name() == "RequestContext"
}

func typesFromHandler(decl *ast.FuncDecl, info *types.Info, framework string) (types.Type, types.Type) {
	var requestType types.Type
	var responseType types.Type
	bestResponseScore := -1
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if requestType == nil && isBindMethod(selector.Sel.Name, framework) && len(call.Args) > 0 {
			requestType = info.TypeOf(call.Args[0])
		}
		if isJSONMethod(selector.Sel.Name) && len(call.Args) >= 2 {
			candidate := info.TypeOf(call.Args[1])
			if score := responseCallScore(call.Args[0], candidate, info); score > bestResponseScore {
				responseType = candidate
				bestResponseScore = score
			}
		}
		return true
	})
	return requestType, responseType
}

func isBindMethod(name, framework string) bool {
	if framework == "hertz" {
		return name == "BindAndValidate"
	}
	switch name {
	case "ShouldBind", "ShouldBindJSON", "ShouldBindXML", "ShouldBindQuery", "ShouldBindUri",
		"ShouldBindHeader", "ShouldBindYAML", "ShouldBindTOML", "Bind", "BindJSON", "BindXML",
		"BindQuery", "BindUri", "BindHeader", "BindYAML", "BindTOML":
		return true
	default:
		return false
	}
}

func isJSONMethod(name string) bool {
	switch name {
	case "JSON", "IndentedJSON", "SecureJSON", "JSONP", "PureJSON":
		return true
	default:
		return false
	}
}

func typeScore(t types.Type) int {
	if t == nil {
		return -1
	}
	for {
		switch typed := t.(type) {
		case *types.Pointer:
			t = typed.Elem()
		case *types.Alias:
			t = types.Unalias(typed)
		default:
			goto scored
		}
	}
scored:
	switch typed := t.(type) {
	case *types.Named:
		return typeScore(typed.Underlying())
	case *types.Struct:
		return 3
	case *types.Slice, *types.Array:
		return 2
	case *types.Map:
		return 1
	default:
		return 0
	}
}

func responseCallScore(status ast.Expr, responseType types.Type, info *types.Info) int {
	typeScore := typeScore(responseType)
	if typeScore < 0 {
		return typeScore
	}

	statusScore := 100
	if value := info.Types[status].Value; value != nil {
		if code, exact := constant.Int64Val(value); exact {
			statusScore = 0
			if code >= 200 && code < 300 {
				statusScore = 200
			}
		}
	}

	return statusScore + typeScore
}

func schemaSet(schema spec.Schema) bool {
	return schema.Type != "" || schema.Ref != "" || len(schema.AllOf) > 0 ||
		len(schema.OneOf) > 0 || len(schema.AnyOf) > 0
}

func renderGeneratedFile(packageName string, handlers []generatedHandler) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("// Code generated by openapi-gen. DO NOT EDIT.\n\n")
	fmt.Fprintf(&output, "package %s\n\n", packageName)
	output.WriteString("import (\n")
	output.WriteString("\t\"encoding/json\"\n\n")
	output.WriteString("\topenapianalyzer \"github.com/thiagozs/go-openapi-gen/analyzer\"\n")
	output.WriteString("\topenapispec \"github.com/thiagozs/go-openapi-gen/spec\"\n")
	output.WriteString(")\n\n")
	output.WriteString("func openapiGeneratedSchema(raw string) openapispec.Schema {\n")
	output.WriteString("\tvar schema openapispec.Schema\n")
	output.WriteString("\tif err := json.Unmarshal([]byte(raw), &schema); err != nil {\n")
	output.WriteString("\t\tpanic(err)\n\t}\n\treturn schema\n}\n\n")
	output.WriteString("func init() {\n")
	for _, handler := range handlers {
		requestJSON, err := json.Marshal(handler.request)
		if err != nil {
			return nil, err
		}
		responseJSON, err := json.Marshal(handler.response)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "\topenapianalyzer.RegisterGeneratedHandlerSchema(%q, openapianalyzer.HandlerSchema{\n", handler.id)
		fmt.Fprintf(&output, "\t\tRequestSchema: openapiGeneratedSchema(%s),\n", strconv.Quote(string(requestJSON)))
		fmt.Fprintf(&output, "\t\tResponseSchema: openapiGeneratedSchema(%s),\n", strconv.Quote(string(responseJSON)))
		output.WriteString("\t})\n")
	}
	output.WriteString("}\n")

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w\n%s", err, output.String())
	}
	return formatted, nil
}
