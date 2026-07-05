// Package emptyiface provides a go/analysis analyzer enforcing the gomatic Go
// standard that the empty interface is written as any, not interface{}. It offers
// a mechanical fix, suppressed when the rewrite would be unsafe: when the name
// any at the reported position does not resolve to the universe-scope any (it is
// shadowed by another declaration), or when a comment lies inside the interface
// braces and would be destroyed by the rewrite. The diagnostic is always emitted.
package emptyiface

import (
	"go/ast"
	"go/token"
	"go/types"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const message = "prefer any to the empty interface{}"

// Analyzer reports every literal empty interface{} and offers to rewrite it to any.
var Analyzer = &analysis.Analyzer{
	Name:     "emptyiface",
	Doc:      "reports the empty interface{}, which the gomatic Go standard writes as any",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// Registration declares this analyzer to the yze framework.
var Registration = goyze.Registration{
	Name:       "emptyiface",
	Categories: []goyze.Category{"modern-go"},
	URL:        "https://docs.gomatic.dev/yze/emptyiface",
	Analyzer:   Analyzer,
}

// run reports each empty interface type with a fix replacing it with any.
func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.InterfaceType)(nil)}, func(n ast.Node) {
		if it := n.(*ast.InterfaceType); len(it.Methods.List) == 0 {
			report(pass, it)
		}
	})
	return nil, nil
}

// report emits the diagnostic for an empty interface, with the any-rewrite fix
// when it is safe to offer.
func report(pass *analysis.Pass, it *ast.InterfaceType) {
	pass.Report(analysis.Diagnostic{
		Pos:            it.Pos(),
		End:            it.End(),
		Message:        message,
		SuggestedFixes: fixes(pass, it),
	})
}

// fixes returns the any-rewrite fix, or nil when the rewrite is unsafe: any is
// shadowed at the interface position, or a comment inside the braces would be lost.
func fixes(pass *analysis.Pass, it *ast.InterfaceType) []analysis.SuggestedFix {
	if !anyIsUniverse(pass, it.Pos()) || hasCommentInside(pass, it) {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message:   "replace interface{} with any",
		TextEdits: []analysis.TextEdit{{Pos: it.Pos(), End: it.End(), NewText: []byte("any")}},
	}}
}

// anyIsUniverse reports whether the name any at pos resolves to the
// universe-scope any, i.e. it is not shadowed by an enclosing declaration.
func anyIsUniverse(pass *analysis.Pass, pos token.Pos) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent("any", pos)
	return obj == types.Universe.Lookup("any")
}

// hasCommentInside reports whether any comment lies within the interface type's
// source span, where the rewrite to any would destroy it.
func hasCommentInside(pass *analysis.Pass, it *ast.InterfaceType) bool {
	for _, group := range enclosingFile(pass, it.Pos()).Comments {
		if group.Pos() > it.Pos() && group.End() < it.End() {
			return true
		}
	}
	return false
}

// enclosingFile returns the file of pass.Files whose extent contains pos. Every
// reported node comes from pass.Files, so a containing file always exists.
func enclosingFile(pass *analysis.Pass, pos token.Pos) *ast.File {
	var found *ast.File
	for _, file := range pass.Files {
		if file.FileStart <= pos && pos < file.FileEnd {
			found = file
		}
	}
	return found
}
