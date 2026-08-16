// Package emptyiface provides a go/analysis analyzer enforcing the gomatic Go
// standard that the empty interface is written as any, not interface{}.
//
// Both halves of the verdict are asked of the judged file. go/types answers
// whether the interface is empty, and the file's own tokens answer whether it
// SAYS so: an interface is reported when it is empty AND every element between
// its braces is the word any, in however many parentheses. interface{},
// interface{ any } and interface{ (any) } are the same type and all three are
// reported — a rule keyed on how many elements the braces hold reports the first
// and lets the others walk out of the standard for one extra token — while an
// interface carrying a method, a type term or a non-empty embedded constraint —
// interface{ Do() }, interface{ ~int }, interface{ comparable } — is not the
// empty interface and is never reported.
//
// An interface whose elements are DECLARED names — interface{ Marker },
// interface{ pkg.Marker }, an embedded alias, an embedded literal — is outside
// the rule even where the type it denotes is empty today, and the withdrawal is
// a requirement rather than a taste. Such an interface is not a SPELLING of the
// empty interface: it is a composition of named parts that happens to be empty,
// so the rule's reason does not reach it. The rewrite would delete the name,
// which across packages deletes the last use of an import and leaves a file that
// does not compile — the framework applies fixes through gofmt, which removes no
// imports. And the verdict would not be a function of the judged file at all: a
// name declared once per GOOS behind a //go:build constraint resolves to an empty
// interface on one platform and a non-empty one on another, so one untagged file
// would be reported on one machine and silent on the next, and the remedy taken
// on the first would break the build on the second.
//
// The word any is read as a word for that same reason. It is an ordinary
// predeclared identifier and a package may declare its own, so a rule asking
// which declaration it resolves to would report interface{ any } on the platform
// where the package's own declaration is not built and stay silent on the one
// where it is — the same divergence, arriving through the identifier the rule is
// named after. Nothing is lost by reading it as a word, because a package that
// rebinds any to some other type fails the emptiness half.
//
// It offers a mechanical fix, suppressed in the three cases where the rewrite
// would not be safe: the word any, written at the reported position, no longer
// denotes the empty interface (a declaration in scope has rebound it — an alias
// to interface{} has not), a comment lies inside the interface braces and would
// be destroyed by the rewrite, or the file's effective language version is below
// go1.18, where any is not yet predeclared and the rewritten file does not
// compile. A version the loader leaves unstated is not evidence the rewrite is
// illegal, so the fix stands. Three is the whole list because of what is reported
// rather than because of what is guarded: the span the rewrite replaces holds
// nothing but braces, parentheses and the word any, so it can carry away no
// declaration, no import and no meaning the file had. A fourth case would be a
// shape reported in error.
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
	return ok && iface.Empty() && isSpeltInPlace(it)
}

// isSpeltInPlace reports whether the interface says it is empty in its own
// tokens: every element between the braces is the word any, of which the empty
// element list is the degenerate case. An element naming a declaration — here, in
// another file of the package, or in another package — carries the emptiness
// somewhere else, where a build tag may move it and where the rewrite deleting
// the name may take an import with it.
func isSpeltInPlace(it *ast.InterfaceType) bool {
	for _, element := range it.Methods.List {
		if !isSpeltAny(element.Type) {
			return false
		}
	}
	return true
}

// isSpeltAny reports whether the interface element is the word any, in however
// many parentheses the grammar allows and gofmt preserves. The test is the
// SPELLING and deliberately not what the name resolves to: any is an ordinary
// predeclared identifier, a package may declare its own, and a package that
// declares one in a file behind a //go:build constraint would otherwise make one
// unchanged interface{ any } report on one platform and not on the next.
// Nothing is lost by asking the narrower question, because the caller has already
// asked go/types whether the type is empty — a package that rebinds any to
// something else fails that half.
func isSpeltAny(element ast.Expr) bool {
	name, ok := ast.Unparen(element).(*ast.Ident)
	return ok && name.Name == replacement
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

// fixes returns the any-rewrite fix, or nil when the rewrite is unsafe: the word
// any has been rebound to some other type at this position, a comment inside the
// braces would be lost, or the file's language version does not predeclare any.
func fixes(pass *analysis.Pass, it *ast.InterfaceType) []analysis.SuggestedFix {
	if !anyMeansTheEmptyInterface(pass, it.Pos()) || hasCommentInside(pass, it) || !anyIsSpellable(pass, it.Pos()) {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message:   fixTitle,
		TextEdits: []analysis.TextEdit{{Pos: it.Pos(), End: it.End(), NewText: []byte(replacement)}},
	}}
}

// anyMeansTheEmptyInterface reports whether the word any, written at pos, still
// denotes the empty interface. The universe declaration is the usual answer, and
// a package or a function may declare its own — an alias to interface{} denotes
// the same type and the rewrite is safe, while an alias to anything else, or to a
// named type, would change what the file says.
func anyMeansTheEmptyInterface(pass *analysis.Pass, pos token.Pos) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent(replacement, pos)
	return obj == types.Universe.Lookup(replacement) ||
		(obj != nil && types.Identical(obj.Type(), types.Universe.Lookup(replacement).Type()))
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
