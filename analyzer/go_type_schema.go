package analyzer

import (
	"go/ast"
	"go/constant"
	"go/types"
	"reflect"
	"strings"

	"github.com/thiagozs/go-openapi-gen/spec"
)

// GenerateSchemaFromGoExpr preserves concrete value types inside free-form
// map literals such as gin.H{"data": endpoints}.
func (sg *SchemaGenerator) GenerateSchemaFromGoExpr(expr ast.Expr, info *types.Info) spec.Schema {
	if expr == nil {
		return spec.Schema{}
	}
	if info == nil {
		return spec.Schema{}
	}

	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return sg.GenerateSchemaFromGoType(info.TypeOf(expr))
	}
	if !isFreeFormStringMap(info.TypeOf(literal)) {
		return sg.GenerateSchemaFromGoType(info.TypeOf(expr))
	}

	properties := make(map[string]spec.Schema, len(literal.Elts))
	for _, element := range literal.Elts {
		entry, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return sg.GenerateSchemaFromGoType(info.TypeOf(expr))
		}
		value := info.Types[entry.Key].Value
		if value == nil {
			return sg.GenerateSchemaFromGoType(info.TypeOf(expr))
		}
		if value.Kind() != constant.String {
			return sg.GenerateSchemaFromGoType(info.TypeOf(expr))
		}
		properties[constant.StringVal(value)] = sg.GenerateSchemaFromGoExpr(entry.Value, info)
	}

	return spec.Schema{Type: "object", Properties: properties}
}

func isFreeFormStringMap(t types.Type) bool {
	for {
		switch typed := t.(type) {
		case *types.Alias:
			t = types.Unalias(typed)
		case *types.Named:
			t = typed.Underlying()
		default:
			mapping, ok := t.(*types.Map)
			if !ok {
				return false
			}
			key, ok := mapping.Key().Underlying().(*types.Basic)
			if !ok {
				return false
			}
			if key.Info()&types.IsString == 0 {
				return false
			}
			_, freeForm := mapping.Elem().Underlying().(*types.Interface)
			return freeForm
		}
	}
}

// GenerateSchemaFromGoType generates an OpenAPI schema from type information
// produced by go/types. Unlike reflection, this also works for named types in
// the application being inspected without requiring a runtime value of them.
func (sg *SchemaGenerator) GenerateSchemaFromGoType(t types.Type) spec.Schema {
	return sg.generateSchemaFromGoType(t, make(map[types.Type]bool))
}

func (sg *SchemaGenerator) generateSchemaFromGoType(t types.Type, processing map[types.Type]bool) spec.Schema {
	if t == nil {
		return spec.Schema{}
	}

	if processing[t] {
		return spec.Schema{Type: "object", Description: "Circular reference to " + types.TypeString(t, nil)}
	}
	processing[t] = true
	defer delete(processing, t)

	switch typed := t.(type) {
	case *types.Alias:
		return sg.generateSchemaFromGoType(types.Unalias(typed), processing)
	case *types.Named:
		if obj := typed.Obj(); obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "time" && obj.Name() == "Time" {
			return spec.Schema{Type: "string", Format: "date-time"}
		}
		return sg.generateSchemaFromGoType(typed.Underlying(), processing)
	case *types.Pointer:
		return sg.generateSchemaFromGoType(typed.Elem(), processing)
	case *types.Basic:
		return schemaForBasicGoType(typed)
	case *types.Slice:
		item := sg.generateSchemaFromGoType(typed.Elem(), processing)
		return spec.Schema{Type: "array", Items: &item}
	case *types.Array:
		item := sg.generateSchemaFromGoType(typed.Elem(), processing)
		return spec.Schema{Type: "array", Items: &item}
	case *types.Map:
		value := sg.generateSchemaFromGoType(typed.Elem(), processing)
		return spec.Schema{Type: "object", AdditionalProperties: &value}
	case *types.Struct:
		return sg.schemaForGoStruct(typed, processing)
	case *types.Interface:
		return spec.Schema{}
	default:
		return spec.Schema{Type: "object", Description: "Unsupported type: " + types.TypeString(t, nil)}
	}
}

func schemaForBasicGoType(t *types.Basic) spec.Schema {
	info := t.Info()
	switch {
	case info&types.IsBoolean != 0:
		return spec.Schema{Type: "boolean"}
	case info&types.IsInteger != 0:
		return spec.Schema{Type: "integer"}
	case info&types.IsFloat != 0:
		return spec.Schema{Type: "number"}
	case info&types.IsString != 0:
		return spec.Schema{Type: "string"}
	default:
		return spec.Schema{Type: "object"}
	}
}

func (sg *SchemaGenerator) schemaForGoStruct(t *types.Struct, processing map[types.Type]bool) spec.Schema {
	schema := spec.Schema{
		Type:       "object",
		Properties: make(map[string]spec.Schema),
		Required:   []string{},
	}

	for i := 0; i < t.NumFields(); i++ {
		field := t.Field(i)
		if !field.Exported() {
			continue
		}

		tags := reflect.StructTag(t.Tag(i))
		jsonName, jsonOptions := parseJSONTag(tags.Get("json"))
		if jsonName == "-" {
			continue
		}
		if jsonName == "" {
			jsonName = sg.toSnakeCase(field.Name())
		}

		fieldSchema := sg.generateSchemaFromGoType(field.Type(), processing)
		validation := tags.Get("validate")
		if validation == "" {
			validation = tags.Get("binding")
		}
		sg.applyValidationTags(validation, &fieldSchema)
		fieldSchema.Example = emptyToNil(tags.Get("example"))
		fieldSchema.Description = tags.Get("description")
		schema.Properties[jsonName] = fieldSchema

		if strings.Contains(validation, "required") && !strings.Contains(jsonOptions, "omitempty") {
			schema.Required = append(schema.Required, jsonName)
		}
	}

	return schema
}

func parseJSONTag(tag string) (string, string) {
	if tag == "" {
		return "", ""
	}
	parts := strings.SplitN(tag, ",", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func emptyToNil(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
