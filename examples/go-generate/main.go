package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

func main() {
	targetFile := os.Getenv("GOFILE")
	targetLineStr := os.Getenv("GOLINE")
	var targetLine int
	if i, err := strconv.Atoi(targetLineStr); err != nil {
		panic(err)
	} else {
		targetLine = i
	}

	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, targetFile, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	f := fileSet.File(astFile.FileStart)
	lines := f.Lines()

	docs, err := doc.NewFromFiles(fileSet, []*ast.File{astFile}, "./", doc.PreserveAST)
	if err != nil {
		panic(err)
	}

	w := &walker{
		lines:          lines,
		docs:           docs,
		goGenerateLine: targetLine,
	}
	ast.Walk(w, astFile)

	fmt.Println(w.output.String())
}

type walker struct {
	lines          []int        // line position offset mapping
	docs           *doc.Package // package documentation
	goGenerateLine int          // line number of the //go:generate comment

	pendingLine bool            // true if the next type node is a target type
	output      strings.Builder // output buffer
}

func (w *walker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Comment:
		if !n.Pos().IsValid() {
			return w
		}

		// check if the comment is a //go:generate comment
		text := n.Text
		if !strings.HasPrefix(text, "//go:generate") {
			return w
		}
		// check if the comment is on the same line as the $GOLINE
		var line int
		for l, pos := range w.lines {
			if token.Pos(pos) > n.Pos() {
				break
			}
			// $GOLINE env var is 1-based.
			line = l + 1
		}
		if line != w.goGenerateLine {
			return w
		}

		// now we are at the correct line
		w.pendingLine = true
	case *ast.TypeSpec:
		if !w.pendingLine {
			return w
		}

		// found the target type
		w.pendingLine = false

		// get the type name
		name := n.Name.String()
		strct, ok := n.Type.(*ast.StructType)
		if !ok {
			return w
		}
		w.output.WriteString(name)

		// find the type documentation
		var typeDoc *doc.Type
		for _, t := range w.docs.Types {
			if t.Name == name {
				typeDoc = t
				break
			}
		}
		if typeDoc != nil {
			w.output.WriteString(" - ")
			w.output.WriteString(typeDoc.Doc)
		}

		for _, field := range strct.Fields.List {
			if len(field.Names) == 0 {
				// embedded field
				continue
			}
			var names []string
			for _, name := range field.Names {
				names = append(names, name.String())
			}
			namesStr := strings.Join(names, ", ")
			fieldType := field.Type.(*ast.Ident).Name
			var fieldDoc string
			if fd := field.Doc; fd != nil {
				fieldDoc = fd.Text()
			}

			w.output.WriteString(" - ")
			w.output.WriteString(namesStr)
			w.output.WriteString(" ")
			w.output.WriteString(fmt.Sprintf("(%s)", fieldType))
			w.output.WriteString(" - ")
			w.output.WriteString(fieldDoc)
		}

	}
	return w
}
