package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

func typeToString(expr ast.Expr) string {
	if expr == nil {
		return "any"
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	}
	return "any"
}

// RouteMeta represents metadata about an API route.
type RouteMeta struct {
	Method      string
	Path        string
	Name        string
	Input       string
	Output      string
	Description string
}

// StructMeta represents metadata about a Go struct.
type StructMeta struct {
	Name        string
	Fields      []FieldMeta
	Description string
}

// FieldMeta represents a field in a struct.
type FieldMeta struct {
	Name string
	Type string
	Tag  string
}

// Metadata holds all parsed information.
type Metadata struct {
	Routes  []RouteMeta
	Structs []StructMeta
}

// Parse parses a directory to extract route and struct metadata.
func Parse(dir string) (*Metadata, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory: %w", err)
	}

	meta := &Metadata{
		Routes:  make([]RouteMeta, 0),
		Structs: make([]StructMeta, 0),
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				// Find struct definitions
				if typeSpec, ok := n.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						description := ""
						if typeSpec.Doc != nil {
							description = strings.TrimSpace(typeSpec.Doc.Text())
						} else if genDecl, ok := file.Decls[0].(*ast.GenDecl); ok && genDecl.Doc != nil {
							// Try to get doc from parent GenDecl if it's a grouped 'type'
							description = strings.TrimSpace(genDecl.Doc.Text())
						}

						structMeta := StructMeta{
							Name:        typeSpec.Name.Name,
							Description: description,
						}
						// ... rest of field parsing logic same ...
						for _, field := range structType.Fields.List {
							if len(field.Names) > 0 {
								tag := ""
								if field.Tag != nil {
									tag = field.Tag.Value
								}

								fieldTypeStr := typeToString(field.Type)

								structMeta.Fields = append(structMeta.Fields, FieldMeta{
									Name: field.Names[0].Name,
									Type: fieldTypeStr,
									Tag:  tag,
								})
							}
						}
						meta.Structs = append(meta.Structs, structMeta)
					}
				}

				// Find route registrations (e.g. r.Get, router.Post, etc.)
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						methodName := sel.Sel.Name
						validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "WS": true}
						if validMethods[methodName] && len(call.Args) >= 2 {
							if basicLit, ok := call.Args[0].(*ast.BasicLit); ok {
								path := strings.Trim(basicLit.Value, "\"")

								// Generate a nice name for the route
								routeName := generateRouteName(methodName, path)

								input := "any"
								output := "any"
								description := ""

								// Find description from comments above the call
								callPos := fset.Position(call.Pos())
								for _, group := range file.Comments {
									groupPos := fset.Position(group.End())
									if groupPos.Line == callPos.Line-1 {
										description = strings.TrimSpace(group.Text())
										break
									}
								}

								// Try to find the handler function literal and scan for types
								handlerIdx := 1
								if methodName == "WS" {
									// WS might have different args
								}

								arg := call.Args[handlerIdx]
								var handlerFunc *ast.FuncLit

								// Handle func(c *http.Context) { ... }
								if fl, ok := arg.(*ast.FuncLit); ok {
									handlerFunc = fl
								} else if _, ok := arg.(*ast.Ident); ok {
									// It's a named function
								}

								if handlerFunc != nil {
									in, out := scanHandlerForTypes(handlerFunc)
									if in != "" {
										input = in
									}
									if out != "" {
										output = out
									}
								}

								meta.Routes = append(meta.Routes, RouteMeta{
									Method:      methodName,
									Path:        path,
									Name:        routeName,
									Input:       input,
									Output:      output,
									Description: description,
								})
							}
						}
					}
				}

				return true
			})
		}
	}

	return meta, nil
}

func generateRouteName(method, path string) string {
	parts := strings.Split(path, "/")
	var cleanParts []string
	for _, p := range parts {
		if p == "" || strings.HasPrefix(p, "{") {
			continue
		}
		cleanParts = append(cleanParts, p)
	}

	if len(cleanParts) == 0 {
		return strings.ToLower(method) + "Index"
	}

	name := strings.ToLower(method)
	for _, p := range cleanParts {
		if len(p) > 0 {
			name += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return name
}

func scanHandlerForTypes(fn *ast.FuncLit) (input, output string) {
	varTypes := make(map[string]string)
	input = "any"
	output = "any"

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Track variable types to resolve Bind/JSON calls
		if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
			for i, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
					if comp, ok := assign.Rhs[i].(*ast.CompositeLit); ok {
						varTypes[ident.Name] = typeToString(comp.Type)
					}
				}
			}
		}

		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				method := sel.Sel.Name
				// Input: c.Bind(&data)
				if (method == "Bind" || method == "BindAndValidate") && len(call.Args) > 0 {
					arg := call.Args[0]
					if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
						if ident, ok := unary.X.(*ast.Ident); ok {
							if t, ok := varTypes[ident.Name]; ok {
								input = t
							}
						}
					}
				}
				// Output: c.JSON(200, data) or c.Success(data)
				if (method == "JSON" || method == "Success" || method == "Created" || method == "CreatedJSON") && len(call.Args) > 0 {
					idx := 0
					if method == "JSON" && len(call.Args) > 1 {
						idx = 1
					}
					arg := call.Args[idx]
					if comp, ok := arg.(*ast.CompositeLit); ok {
						output = typeToString(comp.Type)
					} else if ident, ok := arg.(*ast.Ident); ok {
						if t, ok := varTypes[ident.Name]; ok {
							output = t
						}
					}
				}
				if method == "NoContent" {
					output = "void"
				}
			}
		}
		return true
	})

	return input, output
}
