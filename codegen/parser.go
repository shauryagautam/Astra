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
	Method string
	Path   string
	Name   string
	Input  string
	Output string
}

// StructMeta represents metadata about a Go struct.
type StructMeta struct {
	Name   string
	Fields []FieldMeta
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
						structMeta := StructMeta{Name: typeSpec.Name.Name}
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

				// Find route registrations (simplistic approach looking for e.g. app.POST)
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						methodName := sel.Sel.Name
						if (methodName == "GET" || methodName == "POST" || methodName == "PUT" || methodName == "DELETE" || methodName == "WS") && len(call.Args) >= 2 {
							if basicLit, ok := call.Args[0].(*ast.BasicLit); ok {
								path := strings.Trim(basicLit.Value, "\"")

								// Clean up route name
								routeName := strings.ReplaceAll(path, "/", "_")
								routeName = strings.ReplaceAll(routeName, "{", "")
								routeName = strings.ReplaceAll(routeName, "}", "")
								routeName = strings.Trim(routeName, "_")
								if routeName == "" {
									routeName = "index"
								}

								input := "unknown"
								output := "unknown"

								if funcLit, ok := call.Args[1].(*ast.FuncLit); ok {
									varTypes := make(map[string]string)
									ast.Inspect(funcLit.Body, func(nn ast.Node) bool {
										if decl, ok := nn.(*ast.DeclStmt); ok {
											if gen, ok := decl.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
												for _, spec := range gen.Specs {
													if valSpec, ok := spec.(*ast.ValueSpec); ok {
														typeName := typeToString(valSpec.Type)
														for _, name := range valSpec.Names {
															varTypes[name.Name] = typeName
														}
													}
												}
											}
										}
										if assign, ok := nn.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
											if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
												if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
													if compLit, ok := assign.Rhs[0].(*ast.CompositeLit); ok {
														varTypes[ident.Name] = typeToString(compLit.Type)
													}
												}
											}
										}

										if c, ok := nn.(*ast.CallExpr); ok {
											if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
												if sel.Sel.Name == "Bind" || sel.Sel.Name == "BindAndValidate" {
													if len(c.Args) > 0 {
														if unary, ok := c.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
															if ident, ok := unary.X.(*ast.Ident); ok {
																if t, ok := varTypes[ident.Name]; ok {
																	input = t
																}
															}
														}
													}
												}
												if sel.Sel.Name == "JSON" || sel.Sel.Name == "Created" {
													if len(c.Args) > 0 {
														if compLit, ok := c.Args[0].(*ast.CompositeLit); ok {
															output = typeToString(compLit.Type)
														} else if ident, ok := c.Args[0].(*ast.Ident); ok {
															if t, ok := varTypes[ident.Name]; ok {
																output = t
															}
														}
													}
												}
											}
										}
										return true
									})
								}

								meta.Routes = append(meta.Routes, RouteMeta{
									Method: methodName,
									Path:   path,
									Name:   routeName,
									Input:  input,
									Output: output,
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
