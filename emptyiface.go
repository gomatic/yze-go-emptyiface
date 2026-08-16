// Package emptyiface provides a go/analysis analyzer enforcing the gomatic Go
// standard that the empty interface is written as any, not interface{}.
//
// The verdict is taken from the TYPE and never from the token count, and only
// where the judged file's own tokens carry it: an interface is reported when
// go/types calls it empty AND every element between its braces is the
// predeclared any. interface{} and interface{ any } denote the same type and
// both are reported — a rule keyed on how many elements the braces hold reports
// the first and lets the second walk out of the standard for one extra token —
// while an interface carrying a method, a type term or a non-empty embedded
// constraint — interface{ Do() }, interface{ ~int }, interface{ comparable } —
// is not the empty interface and is never reported.
//
// An interface whose elements are DECLARED names — interface{ Marker },
// interface{ pkg.Marker } — is outside the rule even where the type it denotes
// is empty today, and the withdrawal is a requirement rather than a taste. Such
// an interface is not a SPELLING of the empty interface: it is a composition of
// named parts that happens to be empty, so the rule's reason does not reach it.
// The rewrite would delete the name, which across packages deletes the last use
// of an import and leaves a file that does not compile — the framework applies
// fixes through gofmt, which removes no imports. And the verdict would not be a
// function of the judged file at all: a name declared once per GOOS behind a
// //go:build constraint resolves to an empty interface on one platform and a
// non-empty one on another, so one untagged file would be reported on one
// machine and silent on the next, and the remedy taken on the first would break
// the build on the second.
//
// It offers a mechanical fix, suppressed in the three cases where the rewrite
// would not be safe: the name any at the reported position does not resolve to
// the universe-scope any (it is shadowed by another declaration), a comment lies
// inside the interface braces and would be destroyed by the rewrite, or the
// file's effective language version is below go1.18, where any is not yet
// predeclared and the rewritten file does not compile. A version the loader
// leaves unstated is not evidence the rewrite is illegal, so the fix stands.
// Three is the whole list because of what is reported rather than because of
// what is guarded: the span the rewrite replaces holds nothing but braces and
// the predeclared any, so it can carry away no declaration, no import and no
// meaning the file had. A fourth case would be a shape reported in error.
//
// The diagnostic is emitted for every empty interface spelt in place, in every
// file, test files included: the rule is about how a type is spelt, and no test
// idiom needs the long spelling.
package emptyiface

import (
	"go/ast"
	"go/token"
	"go/types"
	"go/version"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// message is the diagnostic's whole user-facing sentence.
const message = "prefer any to the empty interface{}"

// fixTitle is what an editor's quick-fix menu offers the user.
const fixTitle = "replace interface{} with any"

// replacement is the spelling the standard requires and the fix writes.
const replacement = "any"

// anyVersion is the first language version predeclaring the identifier any.
const anyVersion = "go1.18"

// Analyzer reports every empty interface, however spelt, and offers to rewrite it to any.
var Analyzer = &analysis.Analyzer{
	Name:     "emptyiface",
	Doc:      "reports the empty interface{}, which the gomatic Go standard writes as any",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// Registration declares this analyzer to the yze framework. It carries "types"
// beside "modern-go" because the rule is about how a type is written in a
// signature, which is the axis namedtypes and anonstruct are selected on: a
// category nothing else belongs to is a category nobody can filter by.
var Registration = goyze.Registration{
	Name:       "emptyiface",
	Categories: []goyze.Category{"modern-go", "types"},
	URL:        "https://docs.gomatic.dev/yze/emptyiface",
	Analyzer:   Analyzer,
}

// run reports each empty interface type with a fix replacing it with any.
func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.InterfaceType)(nil)}, func(n ast.Node) {
		if it := n.(*ast.InterfaceType); isEmptyInterface(pass, it) {
			report(pass, it)
		}
	})
	return nil, nil
}

// isEmptyInterface reports whether the interface type expression is the empty
// interface SPELT OUT in place. go/types answers the first half and the syntax
// does not: interface{ any } has one element and is the empty interface, while
// interface{ comparable } has one element and is not. The syntax answers the
// second half and go/types does not: interface{ Marker } is the same type as
// interface{} and is a composition of named parts rather than a spelling of it.
func isEmptyInterface(pass *analysis.Pass, it *ast.InterfaceType) bool {
	iface, ok := pass.TypesInfo.TypeOf(it).(*types.Interface)
	return ok && iface.Empty() && isSpeltInPlace(pass, it)
}

// isSpeltInPlace reports whether the interface says it is empty in its own
// tokens: every element between the braces is the predeclared any, of which the
// empty element list is the degenerate case. An element naming a declaration —
// here, in another file of the package, or in another package — carries the
// emptiness somewhere else, where a build tag may move it and where the rewrite
// deleting the name may take an import with it.
func isSpeltInPlace(pass *analysis.Pass, it *ast.InterfaceType) bool {
	for _, element := range it.Methods.List {
		if !isPredeclaredAny(pass, element.Type) {
			return false
		}
	}
	return true
}

// isPredeclaredAny reports whether the interface element is the identifier any
// resolved to its universe-scope declaration. Object identity is the test to
// make: every empty interface has the same TYPE as any, so a comparison of types
// would readmit the declared names this rule withdraws from.
func isPredeclaredAny(pass *analysis.Pass, element ast.Expr) bool {
	name, ok := element.(*ast.Ident)
	return ok && pass.TypesInfo.Uses[name] == types.Universe.Lookup(replacement)
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
// shadowed at the interface position, a comment inside the braces would be lost,
// or the file's language version does not predeclare any.
func fixes(pass *analysis.Pass, it *ast.InterfaceType) []analysis.SuggestedFix {
	if !anyIsUniverse(pass, it.Pos()) || hasCommentInside(pass, it) || !anyIsSpellable(pass, it.Pos()) {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message:   fixTitle,
		TextEdits: []analysis.TextEdit{{Pos: it.Pos(), End: it.End(), NewText: []byte(replacement)}},
	}}
}

// anyIsUniverse reports whether the name any at pos resolves to the
// universe-scope any, i.e. it is not shadowed by an enclosing declaration.
func anyIsUniverse(pass *analysis.Pass, pos token.Pos) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent(replacement, pos)
	return obj == types.Universe.Lookup(replacement)
}

// anyIsSpellable reports whether the file containing pos compiles the identifier
// any. types.Universe holds it at every language version, so the shadowing test
// above reports SAFE in a module the compiler rejects; the version is the second
// question and this is where it is asked. A version the loader left unstated is
// not evidence against the rewrite, so it is treated as spellable.
func anyIsSpellable(pass *analysis.Pass, pos token.Pos) bool {
	v := effectiveVersion(pass, pos)
	return !version.IsValid(v) || version.Compare(v, anyVersion) >= 0
}

// effectiveVersion returns the language version in force for the file containing
// pos: the file's own, when the type checker recorded one, because a //go:build
// constraint may raise a file above the module's directive; the package's
// otherwise.
func effectiveVersion(pass *analysis.Pass, pos token.Pos) string {
	if file := enclosingFile(pass, pos); file != nil {
		if v := pass.TypesInfo.FileVersions[file]; v != "" {
			return v
		}
	}
	return pass.Pkg.GoVersion()
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
