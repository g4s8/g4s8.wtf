+++ 
date = 2024-02-02T23:50:20+04:00
title = "Building a Go Documentation Tool with go generate and AST"
description = "Explore how to automate Go documentation using go generate and AST parsing. This guide covers the development of a tool for extracting and generating documentation directly from Go source code, showcasing the practical use of Go's built-in capabilities. Ideal for developers seeking to enhance their Go tooling and coding efficiency."
slug = "go-generate-and-ast"
tags = []
categories = ["go"]
keywords = ["go", "ast", "generate"]
+++

Some time ago, I created a Go tool, [github.com/g4s8/envdoc](https://github.com/g4s8/envdoc), which runs as part of go generate.
It parses Go source files, extracts documentation for struct fields annotated with env tags,
and generates documentation in markdown, HTML, or plaintext formats. This was an enlightening journey,
as I had not previously engaged with Go generators nor worked with Go source file parsers.
In this post, I will share my learnings from this experience and demonstrate how to create Go generators and AST parsers.

Let's construct a simple tool that, when run by the `go generate` command from a `//go:generate` instruction,
parses the struct following this comment and prints all fields and documentation to stdout.
The project structure for this example is as follows:

```txt
|- go.mod
|- go.sum
|- main.go (the tool's main file)
|-|
  |- .testfiles (targets we will test on)
  |-|
    |- target.go (the target file to extract field comments from)
```

Here's an example of the `.testfiles/target.go` struct we'll use in this article:

```go
package main

// Config is a configuration of the target server
//
//go:generate go run ../
type Config struct {
	// Host is a host name or IP address of the target server
	Host string
	// Port is a port number of the target server
	Port int
	// Protocol is a protocol of the target server
	Protocol string
	// Timeout is a timeout of the target server
	Timeout int
}
```

Upon executing the `go generate` command, we expect to see the following output:

```txt
$ go generate ./.testfiles/target.go
Config:
 - Host (string) - Host is a host name or IP address of the target server
 - Port (int) - Port is a port number of the target server
 - Protocol (string) - Protocol is a protocol of the target server
 - Timeout (int) - Timeout is a timeout of the target server
```

## Go Generators

The go [generate documentation](https://pkg.go.dev/cmd/go/internal/generate) specifies several environment variables
made available during the process initiated by go generate:

```txt
Go generate sets several variables when it runs the generator:

$GOARCH
    The execution architecture (arm, amd64, etc.)
$GOOS
    The execution operating system (linux, windows, etc.)
$GOFILE
    The base name of the file.
$GOLINE
    The line number of the directive in the source file.
$GOPACKAGE
    The name of the package of the file containing the directive.
$GOROOT
    The GOROOT directory for the 'go' command that invoked the
    generator, containing the Go toolchain and standard library.
$DOLLAR
    A dollar sign.
$PATH
    The $PATH of the parent process, with $GOROOT/bin
    placed at the beginning. This causes generators
    that execute 'go' commands to use the same 'go'
    as the parent 'go generate' command.
```

We are particularly interested in the `$GOFILE` and `$GOLINE` variables: the former is needed to read the target file
and parse it as an AST, and the latter to identify the `go:generate` statement line,
enabling us to process the subsequent struct definition. Let's begin crafting our `main.go` file:

```go
func main() {
	targetFile := os.Getenv("GOFILE")
	targetLineStr := os.Getenv("GOLINE")
	var targetLine int
	if i, err := strconv.Atoi(targetLineStr); err != nil {
		panic(err)
	} else {
		targetLine = i
	}
}
```

## Parse Tokens

Next, we require a few Go packages to parse the target file:
```txt
go/ast
go/doc
go/parser
go/token
```
First, we utilize the `go/token` package to parse the Go source file into lexical tokens.
To parse the target file, we create a `token.FileSet` structure to track all parsed file tokens,
then call `parser.ParseFile`, which returns an `ast.Files` and adds the file tokens into the fileset.
Since we also need to extract comments, we pass the `parser.ParseComments` flag to `parser.ParseFile`:

```go
fileSet := token.NewFileSet()
astFile, err := parser.ParseFile(fileSet, targetFile, nil, parser.ParseComments)
```

## File Line Positions

The subsequent step involves extracting line info. All tokens are stored with position offsets,
necessitating the acquisition of positions for each line's start.
We can obtain this from the `fileSet` by targeting the file's start position:

```go
f := fileSet.File(astFile.Pos())
lines := f.Lines()
```

## Extract Documentation

We must not overlook extracting documentation for fields within our files, achievable via the `go/doc` package:

```go
docs, err := doc.NewFromFiles(fileSet, []*ast.File{astFile}, "./", doc.PreserveAST)
```

The `docs` variable now contains package documentation, which we can utilize to locate documentation lines for struct fields.
It's important to note the `doc.PreserveAST` flag; by default, `doc.NewFromFiles` modifies AST nodes,
but we require them untouched for later analysis.

## AST Visitor

We are now prepared to navigate the AST of the target file and process each node accordingly.
This necessitates the introduction of a new AST visitor struct to retain all pertinent information gleaned from target AST nodes:

```go
type walker struct {
	lines          []int        // Line position offset mapping.
	docs           *doc.Package // Package documentation.
	goGenerateLine int          // Line number of the //go:generate comment.

	pendingLine bool            // True if the next type node is a target type.
	output      strings.Builder // Output buffer.
}
```

### Find the Trigger Comment

The `walker` type must implement the `Visit` method to traverse AST nodes.
This method encapsulates all logic for AST parsing, beginning with comment node processing.
The objective here is to identify the `go:generate` statement line that matches `$GOLINE` and set `pendingLine` to true,
indicating that the subsequent struct type is to be used for extracting field documentation.

```go
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
		// check if the comment is on the same line as $GOLINE
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
	}
	return w
}
```

### Parse Struct Fields

The final and most intricate part involves dealing with the struct type AST node to extract all fields,
their types, and documentation. This process is encapsulated in the `Visit` method within the subsequent case
after comment processing. Here's a streamlined explanation:

```go
case *ast.TypeSpec:
    if !w.pendingLine {
        return w
    }

    // found the target type
    w.pendingLine = false

    // extract the type name
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

    // iterate over and append field names, types, and documentation.
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
```

## Running AST Walker

To execute our AST `walker/inspector`:

```go
w := &walker{
    lines:          lines,
    docs:           docs,
    goGenerateLine: targetLine,
}
ast.Walk(w, astFile)

fmt.Println(w.output.String())
```

We construct it using the line mappings, parsed package documentation, and the `go:generate` comment line,
then invoke `ast.Walk` with our walker and the astFile. Finally, we print the output:

```txt
$ go generate ./.testfiles/target.go

Config - Config is a configuration of the target server
 - Host (string) - Host is a host name or IP address of the target server
 - Port (int) - Port is a port number of the target server
 - Protocol (string) - Protocol is a protocol of the target server
 - Timeout (int) - Timeout is a timeout of the target server
```

This approach yields the expected result for our examples, demonstrating the power and flexibility of Go's AST
manipulation capabilities for generating custom documentation and other forms of code analysis.

## Conclusion

Through developing the [envdoc](https://github.com/g4s8/envdoc) tool and exploring Go's go generate and AST parsing capabilities,
I've uncovered a powerful approach for automating documentation and enhancing code analysis.
This journey not only illustrates the utility of Go's built-in packages for source code manipulation but also highlights
the potential for creating tools that significantly improve development workflows.

This experience demonstrates the vast possibilities within Go's ecosystem for developers to build sophisticated,
yet straightforward tools that can lead to more maintainable and self-documenting code.
As we look forward, the techniques shared here offer a foundation for further exploration and innovation in code automation and analysis.

I hope this exploration inspires you to delve deeper into Go's features and consider how its tooling can be applied to your own projects,
driving efficiency and code quality to new heights.

## References

 - The final version of this example and target file source code - [examples/go-generate](https://github.com/g4s8/g4s8.wtf/tree/master/examples/go-generate)
 - My documentation generator tool - [github.com/g4s8/envdoc](https://github.com/g4s8/envdoc)
 - `go/token` documentation - [pkg.go.dev/go/token](https://pkg.go.dev/go/token)
 - `go/parser` documentation - [pkg.go.dev/go/parser](https://pkg.go.dev/go/parser)
 - Go generate documentation - [pkg.go.dev/cmd/go/internal/generate](https://pkg.go.dev/cmd/go/internal/generate)
 - Blog post about Go generators - [go.dev/blog/generate](https://go.dev/blog/generate)
