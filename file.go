package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"regexp"
)

var (
	rComment = regexp.MustCompile(`^//.*?@(?i:gotags?|inject_tags?):\s*(.*)$`)
	rInject  = regexp.MustCompile("`.+`$")
	rTags    = regexp.MustCompile(`[\w_]+:"[^"]+"`)
)

type textArea struct {
	Start      int
	End        int
	CurrentTag string
	InjectTag  string
}

func parseFile(inputPath string, xxxSkip []string) (*token.FileSet, ast.Node, error) {
	logf("parsing file %q for inject tag comments", inputPath)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, inputPath, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	ast.Inspect(f, func(n ast.Node) bool {
		structDecl, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structDecl.Fields.List {
			// skip if field is embedded
			if len(field.Names) == 0 {
				continue
			}
			comments := []*ast.Comment{}
			if field.Doc != nil {
				comments = append(comments, field.Doc.List...)
			}
			if field.Comment != nil {
				comments = append(comments, field.Comment.List...)
			}

			injectTags := []string{}
			for _, comment := range comments {
				match := rComment.FindStringSubmatch(comment.Text)
				if len(match) != 2 {
					continue
				}
				injectTags = append(injectTags, match[1])
			}
			if len(injectTags) == 0 {
				continue
			}
			// override tag
			field.Tag.Value = fmt.Sprintf("`%s`", override(field.Tag.Value[1:len(field.Tag.Value)-1], injectTags))
		}
		return true
		// return false // try false
	})
	return fset, f, nil
}

func override(currentTag string, injectTags []string) string {
	if len(injectTags) == 0 {
		return currentTag
	}
	cti := newTagItems(currentTag)
	for _, it := range injectTags {
		iti := newTagItems(it)
		cti = cti.override(iti)
	}
	return cti.format()
}

func writeFile(inputPath string, areas []textArea) (err error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return
	}

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	if err = f.Close(); err != nil {
		return
	}

	// inject custom tags from tail of file first to preserve order
	for i := range areas {
		area := areas[len(areas)-i-1]
		logf("inject custom tag %q to expression %q", area.InjectTag, string(contents[area.Start-1:area.End-1]))
		contents = injectTag(contents, area)
	}
	if err = ioutil.WriteFile(inputPath, contents, 0644); err != nil {
		return
	}

	if len(areas) > 0 {
		logf("file %q is injected with custom tags", inputPath)
	}
	return
}
