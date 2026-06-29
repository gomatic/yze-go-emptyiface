// Package emptyiface provides a go/analysis analyzer enforcing the gomatic Go
// standard that the empty interface is written as any, not interface{}. It offers
// a mechanical fix.
package emptyiface

import (
	"go/ast"

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
	URL:        "https://docs.gomatic.dev/yze/go/emptyiface",
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

// report emits the diagnostic and the any-rewrite fix for an empty interface.
func report(pass *analysis.Pass, it *ast.InterfaceType) {
	pass.Report(analysis.Diagnostic{
		Pos:     it.Pos(),
		End:     it.End(),
		Message: message,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "replace interface{} with any",
			TextEdits: []analysis.TextEdit{{Pos: it.Pos(), End: it.End(), NewText: []byte("any")}},
		}},
	})
}
