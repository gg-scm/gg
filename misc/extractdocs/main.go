// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// extractdocs analyzes gg's source to find command documentation.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/types"
	"html"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/loader"
)

const (
	cmdImportPath  = "zombiezen.com/go/gg/cmd/gg"
	flagImportPath = "zombiezen.com/go/gg/internal/flag"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: extractdocs OUTDIR")
		os.Exit(64)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "extractdocs:", err)
		os.Exit(1)
	}
}

func run(outDir string) error {
	t := time.Now()
	cmds, err := findCommands()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "extractdocs: Found %d commands.\n", len(cmds))
	success := true
	for _, c := range cmds {
		if err := writePage(filepath.Join(outDir, c.name+".md"), c, t); err != nil {
			fmt.Fprintf(os.Stderr, "extractdocs: %v\n", err)
			success = false
		}
	}
	if !success {
		return errors.New("not all pages written")
	}
	return nil
}

func findCommands() ([]*command, error) {
	main, err := build.Default.Import(cmdImportPath, ".", build.FindOnly)
	if err != nil {
		return nil, err
	}
	conf := &loader.Config{
		TypeCheckFuncBodies: func(path string) bool {
			return path == main.ImportPath
		},
	}
	conf.Import(cmdImportPath)
	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}
	pkgInfo := prog.InitialPackages()[0]
	cmds := make(map[types.Object]*command)
	for _, f := range pkgInfo.Files {
		astutil.Apply(f, nil, func(cur *astutil.Cursor) bool {
			call, ok := cur.Node().(*ast.CallExpr)
			if !ok {
				return true
			}
			fn := processFuncExpr(&pkgInfo.Info, call.Fun)
			if fn.pkg == flagImportPath && fn.name == "NewFlagSet" {
				assign, ok := cur.Parent().(*ast.AssignStmt)
				if !ok {
					return true
				}
				assignID := assign.Lhs[0].(*ast.Ident)
				if !ok {
					return true
				}
				assignObj := pkgInfo.ObjectOf(assignID)
				if assignObj == nil {
					return true
				}
				if c := processNewFlagSet(&pkgInfo.Info, call); c != nil {
					cmds[assignObj] = c
				}
			} else if namePos, usagePos, hasArg := isFlagMethod(fn); namePos != -1 {
				id, ok := fn.receiver.(*ast.Ident)
				if !ok {
					return true
				}
				c := cmds[pkgInfo.ObjectOf(id)]
				if c == nil {
					return true
				}
				name := evalString(&pkgInfo.Info, call.Args[namePos])
				if name == "" {
					return true
				}
				arg, usage := unquoteFlagUsage(evalString(&pkgInfo.Info, call.Args[usagePos]), hasArg)
				c.flags = append(c.flags, commandFlag{
					names: []string{name},
					arg:   arg,
					doc:   usage,
				})
			} else if fn.pkg == flagImportPath && fn.receiverName == "FlagSet" && fn.name == "Alias" {
				id, ok := fn.receiver.(*ast.Ident)
				if !ok {
					return true
				}
				c := cmds[pkgInfo.ObjectOf(id)]
				if c == nil {
					return true
				}
				cf := c.findFlag(evalString(&pkgInfo.Info, call.Args[0]))
				if cf == nil {
					return true
				}
				for _, arg := range call.Args[1:] {
					cf.names = append(cf.names, evalString(&pkgInfo.Info, arg))
				}
			}
			return true
		})
	}
	cmdList := make([]*command, 0, len(cmds))
	for _, c := range cmds {
		cmdList = append(cmdList, c)
	}
	sort.Slice(cmdList, func(i, j int) bool {
		return cmdList[i].name < cmdList[j].name
	})
	return cmdList, nil
}

const hugoDateFormat = "2006-01-02 15:04:05Z07:00"

func writePage(path string, c *command, genTime time.Time) error {
	// Get or create front matter.
	var frontMatter map[string]interface{}
	if content, err := ioutil.ReadFile(path); err == nil {
		frontMatter, err = parseFrontMatter(content)
		if err != nil {
			return fmt.Errorf("write page for %s: %v", c.name, err)
		}
	} else if os.IsNotExist(err) {
		frontMatter = map[string]interface{}{
			"title": "gg " + c.name,
			"date":  genTime.Format(hugoDateFormat),
		}
	} else {
		return fmt.Errorf("write page for %s: %v", c.name, err)
	}
	frontMatter["usage"] = c.usage
	frontMatter["synopsis"] = c.synopsis
	frontMatter["lastmod"] = genTime.Format(hugoDateFormat)
	if len(c.aliases) > 0 {
		frontMatter["aliases"] = c.aliases
	} else {
		frontMatter["aliases"] = []string{}
	}

	// Write to file.
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(frontMatter); err != nil {
		return fmt.Errorf("write page for %s: %v", c.name, err)
	}
	if c.doc != "" {
		buf.WriteString("\n")
		buf.WriteString(c.doc)
		buf.WriteString("\n")
	}
	if len(c.flags) > 0 {
		buf.WriteString("\n## Flags\n\n<dl class=\"flag_list\">\n")
		for _, f := range c.flags {
			for _, name := range f.names {
				buf.WriteString("\t<dt>-")
				buf.WriteString(html.EscapeString(name))
				if f.arg != "" {
					buf.WriteString(" ")
					buf.WriteString(html.EscapeString(f.arg))
				}
				buf.WriteString("</dt>\n")
			}
			buf.WriteString("\t<dd>")
			buf.WriteString(html.EscapeString(f.doc))
			buf.WriteString("</dd>\n")
		}
		buf.WriteString("</dl>\n")
	}
	if err := ioutil.WriteFile(path, buf.Bytes(), 0666); err != nil {
		return fmt.Errorf("write page for %s: %v", c.name, err)
	}
	return nil
}

// parseFrontMatter parses Hugo front matter from a file.
//
// Details: https://gohugo.io/content-management/front-matter/
func parseFrontMatter(content []byte) (map[string]interface{}, error) {
	if bytes.HasPrefix(content, []byte("---")) {
		return nil, errors.New("found YAML front matter, want JSON")
	}
	if bytes.HasPrefix(content, []byte("+++")) {
		return nil, errors.New("found TOML front matter, want JSON")
	}
	if !bytes.HasPrefix(content, []byte("{")) {
		return nil, errors.New("could not parse front matter")
	}
	r := bytes.NewReader(content)
	d := json.NewDecoder(r)
	var m map[string]interface{}
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse front matter: %v", err)
	}
	// Next byte after JSON object must be a newline.
	mr := io.MultiReader(d.Buffered(), r)
	var b [1]byte
	if _, err := io.ReadFull(mr, b[:]); err != nil {
		return nil, fmt.Errorf("parse front matter: %v", err)
	}
	if b[0] != '\n' {
		return nil, errors.New("could not parse front matter")
	}
	return m, nil
}

type command struct {
	name     string
	aliases  []string
	usage    string
	synopsis string
	doc      string
	flags    []commandFlag
}

type commandFlag struct {
	names []string
	arg   string
	doc   string
}

func processNewFlagSet(info *types.Info, call *ast.CallExpr) *command {
	usage := evalString(info, call.Args[1])
	fields := strings.Fields(usage)
	if len(fields) < 2 || fields[0] != "gg" || !isCommandName(fields[1]) {
		return nil
	}
	c := &command{
		name:  fields[1],
		usage: usage,
	}
	c.synopsis, c.doc, c.aliases = parseDescription(evalString(info, call.Args[2]))
	return c
}

func (c *command) findFlag(name string) *commandFlag {
	for i := range c.flags {
		for _, n := range c.flags[i].names {
			if n == name {
				return &c.flags[i]
			}
		}
	}
	return nil
}

func evalString(info *types.Info, expr ast.Expr) string {
	v := info.Types[expr].Value
	if v == nil || v.Kind() != constant.String {
		return ""
	}
	return constant.StringVal(v)
}

func isCommandName(word string) bool {
	if word == "ez" {
		return false
	}
	for i := 0; i < len(word); i++ {
		if word[i] < 'a' || 'z' < word[i] {
			return false
		}
	}
	return true
}

func parseDescription(s string) (synopsis, doc string, aliases []string) {
	i := strings.IndexByte(s, '\n')
	if i == -1 {
		return s, "", nil
	}
	synopsis = s[:i]
	doc = strings.TrimSpace(s[i:])
	if strings.HasPrefix(doc, "aliases: ") {
		j := strings.IndexByte(doc, '\n')
		if j == -1 {
			j = len(doc)
		}
		aliases = strings.Split(doc[len("aliases: "):j], ", ")
		doc = strings.TrimSpace(doc[j:])
	}
	return synopsis, strings.Replace(doc, "\t", "", -1), aliases
}

func isFlagMethod(fn funcExpr) (namePos, usagePos int, hasArg bool) {
	if fn.pkg != flagImportPath || fn.receiverName != "FlagSet" {
		return -1, -1, false
	}
	switch fn.name {
	case "Bool":
		return 0, 2, false
	case "Int", "String":
		return 0, 2, true
	case "BoolVar":
		return 1, 3, false
	case "IntVar", "StringVar":
		return 1, 3, true
	case "MultiString":
		return 0, 1, true
	case "MultiStringVar", "Var":
		return 1, 2, true
	default:
		return -1, -1, false
	}
}

func unquoteFlagUsage(usage string, hasArg bool) (name, usage_ string) {
	if i := strings.IndexByte(usage, '`'); i != -1 {
		if j := strings.IndexByte(usage[i+1:], '`'); j != -1 {
			j += i + 1
			name = usage[i+1 : j]
			return name, usage[:i] + name + usage[j+1:]
		}
	}
	if !hasArg {
		return "", usage
	}
	return "string", usage
}

type funcExpr struct {
	pkg          string
	receiverName string
	receiver     ast.Expr
	name         string
}

func processFuncExpr(info *types.Info, expr ast.Expr) funcExpr {
	var obj types.Object
	var recv ast.Expr
	switch expr := expr.(type) {
	case *ast.Ident:
		obj = info.ObjectOf(expr)
		if obj == nil {
			return funcExpr{}
		}
		var pkgPath string
		if pkg := obj.Pkg(); pkg != nil {
			pkgPath = pkg.Path()
		}
		return funcExpr{
			pkg:  pkgPath,
			name: obj.Name(),
		}
	case *ast.SelectorExpr:
		recv = expr.X
		obj = info.ObjectOf(expr.Sel)
		if obj == nil {
			return funcExpr{}
		}
	default:
		return funcExpr{}
	}
	f, ok := obj.Type().(*types.Signature)
	if !ok {
		return funcExpr{}
	}
	var pkgPath string
	if pkg := obj.Pkg(); pkg != nil {
		pkgPath = pkg.Path()
	}
	r := f.Recv()
	if r == nil {
		// Likely a qualified identifier.
		return funcExpr{
			pkg:  pkgPath,
			name: obj.Name(),
		}
	}
	switch rt := r.Type().(type) {
	case *types.Named:
		return funcExpr{
			pkg:          pkgPath,
			receiverName: rt.Obj().Name(),
			receiver:     recv,
			name:         obj.Name(),
		}
	case *types.Pointer:
		tn := rt.Elem().(*types.Named)
		return funcExpr{
			pkg:          pkgPath,
			receiverName: tn.Obj().Name(),
			receiver:     recv,
			name:         obj.Name(),
		}
	default:
		panic("unknown receiver type")
	}
}
