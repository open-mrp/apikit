package openapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
)

type DocReader struct {
	fset *token.FileSet
	docs map[string]map[string]TypeDoc // pkgPath -> typeName -> TypeDoc
}

type TypeDoc struct {
	Doc    string
	Fields map[string]string // fieldName -> doc
}

func NewDocReader() *DocReader {
	return &DocReader{
		fset: token.NewFileSet(),
		docs: make(map[string]map[string]TypeDoc),
	}
}

func (r *DocReader) GetTypeDoc(t reflect.Type) TypeDoc {
	pkgPath := t.PkgPath()
	typeName := t.Name()

	if pkgPath == "" {
		return TypeDoc{}
	}

	// Handle generic types by using the base name for documentation lookup
	if idx := strings.Index(typeName, "["); idx != -1 {
		typeName = typeName[:idx]
	}

	if pkgDocs, ok := r.docs[pkgPath]; ok {
		if typeDoc, ok := pkgDocs[typeName]; ok {
			return typeDoc
		}
	}

	r.loadPackage(pkgPath)

	if pkgDocs, ok := r.docs[pkgPath]; ok {
		return pkgDocs[typeName]
	}

	return TypeDoc{}
}

// loadPackage parses the doc comments of every type in pkgPath, found wherever the go command resolves it from the working directory: the app's own packages, apikit's, or any other dependency.
func (r *DocReader) loadPackage(pkgPath string) {
	dir := packageDir(pkgPath)
	if dir == "" {
		r.docs[pkgPath] = map[string]TypeDoc{}
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		r.docs[pkgPath] = map[string]TypeDoc{}
		return
	}

	pkgDocs := make(map[string]TypeDoc)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(r.fset, filePath, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}

			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				typeName := typeSpec.Name.Name
				typeDoc := TypeDoc{
					Doc:    strings.TrimSpace(gen.Doc.Text()),
					Fields: make(map[string]string),
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if ok {
					for _, field := range structType.Fields.List {
						if len(field.Names) > 0 {
							fieldName := field.Names[0].Name
							typeDoc.Fields[fieldName] = strings.TrimSpace(field.Doc.Text())
						}
					}
				}

				pkgDocs[typeName] = typeDoc
			}
		}
	}

	r.docs[pkgPath] = pkgDocs
}

// packageDir asks the go command where pkgPath's source is, or returns "" when it cannot say.
func packageDir(pkgPath string) string {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", pkgPath).Output() // #nosec G204 - an import path from a reflected type
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
