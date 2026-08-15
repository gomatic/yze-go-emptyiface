package emptyiface

// White-box tests for the matcher's dimension, the fix's version guard and the
// file lookup the fix anchors on — each a question no fixture can put, because
// analysistest offers no way to vary the language version or to withhold the
// type information the matcher reads.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

// TestEnclosingFileFindsTheFileContainingAPosition names enclosingFile's claim.
// "A containing file always exists" is what lets callers use the result without
// a nil check, so the lookup must be by EXTENT rather than by order — with more
// than one file in the pass, returning the first (or the last) would anchor a
// fix in the wrong file, and the edit would land at that offset in a file it
// was never computed against.
func TestEnclosingFileFindsTheFileContainingAPosition(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()

	first, err := parser.ParseFile(fset, "first.go", "package p\n\nvar A any\n", 0)
	require.NoError(t, err)
	second, err := parser.ParseFile(fset, "second.go", "package p\n\nvar B any\nvar C any\n", 0)
	require.NoError(t, err)
	pass := &analysis.Pass{Fset: fset, Files: []*ast.File{first, second}}

	assert.Same(t, first, enclosingFile(pass, first.Pos()), "a position in the first file")
	assert.Same(t, first, enclosingFile(pass, first.FileEnd-1), "including its last byte")
	assert.Same(t, second, enclosingFile(pass, second.Pos()), "a position in the second file")
	assert.Same(t, second, enclosingFile(pass, second.FileEnd-1), "including its last byte")

	assert.Nil(t, enclosingFile(pass, token.NoPos),
		"a position belonging to no file yields nothing rather than an arbitrary file")
}

// languageVersion is a Go language version as the go directive spells it.
type languageVersion string

// checkedAt type-checks the given sources as one package at the given language
// version and returns a pass plus the first empty interface each source declares,
// in the order they were given. The version is what the fix's third guard reads,
// and a fixture has no way to vary it.
func checkedAt(t *testing.T, lang languageVersion, sources ...string) (*analysis.Pass, []*ast.InterfaceType) {
	t.Helper()
	fset := token.NewFileSet()

	files := make([]*ast.File, 0, len(sources))
	ifaces := make([]*ast.InterfaceType, 0, len(sources))
	for i, source := range sources {
		file, err := parser.ParseFile(fset, fmt.Sprintf("p%d.go", i), source, parser.ParseComments)
		require.NoError(t, err)
		files = append(files, file)
		ifaces = append(ifaces, firstInterface(t, file))
	}

	info := &types.Info{
		Types:        map[ast.Expr]types.TypeAndValue{},
		Defs:         map[*ast.Ident]types.Object{},
		Uses:         map[*ast.Ident]types.Object{},
		Scopes:       map[ast.Node]*types.Scope{},
		FileVersions: map[*ast.File]string{},
	}
	conf := types.Config{GoVersion: string(lang)}
	pkg, err := conf.Check("p", fset, files, info)
	require.NoError(t, err)

	return &analysis.Pass{Fset: fset, Files: files, Pkg: pkg, TypesInfo: info}, ifaces
}

// firstInterface returns the first interface type the file declares.
func firstInterface(t *testing.T, file *ast.File) *ast.InterfaceType {
	t.Helper()
	var found *ast.InterfaceType
	ast.Inspect(file, func(n ast.Node) bool {
		if it, ok := n.(*ast.InterfaceType); ok && found == nil {
			found = it
		}
		return true
	})
	require.NotNil(t, found, "the source declares an interface type")
	return found
}

// TestFixIsOfferedOnlyWhereAnyIsALegalSpelling pins the version guard on both
// sides of its boundary. any became predeclared in go1.18; below it the rewrite
// the fix prescribes does not compile, and the author of such a module has no
// spelling that satisfies the rule and no flag to narrow it — the only move left
// is a repository-wide exemption, which is the population the suite exists to
// remove rather than to manufacture.
func TestFixIsOfferedOnlyWhereAnyIsALegalSpelling(t *testing.T) {
	t.Parallel()
	const source = "package p\n\nfunc Store(v interface{}) { _ = v }\n"

	below, its := checkedAt(t, "go1.17", source)
	assert.Nil(t, fixes(below, its[0]), "go1.17 does not predeclare any, so no rewrite is offered")

	at, its := checkedAt(t, "go1.18", source)
	assert.Len(t, fixes(at, its[0]), 1, "go1.18 predeclares any, so the rewrite is offered")

	above, its := checkedAt(t, "go1.26", source)
	assert.Len(t, fixes(above, its[0]), 1, "every later version predeclares it too")
}

// TestAnUnstatedVersionDoesNotWithholdTheFix names the direction the guard must
// NOT take. A loader that reports no version has said nothing about whether the
// rewrite compiles, and treating silence as a prohibition would withdraw the
// remedy from every author whose driver does not fill the field.
func TestAnUnstatedVersionDoesNotWithholdTheFix(t *testing.T) {
	t.Parallel()
	pass, its := checkedAt(t, "", "package p\n\nfunc Store(v interface{}) { _ = v }\n")
	require.Empty(t, pass.Pkg.GoVersion(), "the fixture pins the unstated case")

	assert.Len(t, fixes(pass, its[0]), 1, "silence about the version is not evidence against the rewrite")
}

// TestTheEmptyInterfaceIsDecidedByTheTypeNotTheTokenCount names the matcher's
// dimension. interface{} and interface{ any } are the same type and differ by
// one token; interface{ comparable } differs from interface{ any } by one token
// and is a different type. A rule keyed on what is between the braces gets both
// of those backwards, so the verdict comes from go/types.
func TestTheEmptyInterfaceIsDecidedByTheTypeNotTheTokenCount(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		source  string
		isEmpty bool
	}{
		{"short spelling", "package p\n\ntype T interface{}\n", true},
		{"embedded any", "package p\n\ntype T interface{ any }\n", true},
		{"embedded any twice", "package p\n\ntype T interface {\n\tany\n\tany\n}\n", true},
		{"type term", "package p\n\ntype T interface{ ~int }\n", false},
		{"comparable", "package p\n\ntype T interface{ comparable }\n", false},
		{"method", "package p\n\ntype T interface{ Do() }\n", false},
		{"any beside comparable", "package p\n\ntype T interface {\n\tany\n\tcomparable\n}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pass, its := checkedAt(t, "go1.26", tc.source)
			assert.Equal(t, tc.isEmpty, isEmptyInterface(pass, its[0]))
		})
	}
}

// TestAnUntypedInterfaceIsNotReported names what the matcher does when the type
// checker recorded nothing for the expression. Absent type information says
// nothing about emptiness, and reporting on that absence would report every
// interface in a package that failed to load.
func TestAnUntypedInterfaceIsNotReported(t *testing.T) {
	t.Parallel()
	pass, its := checkedAt(t, "go1.26", "package p\n\ntype T interface{}\n")
	pass.TypesInfo = &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}

	assert.False(t, isEmptyInterface(pass, its[0]), "no recorded type is not evidence of emptiness")
}

// TestTheVersionIsTheFileOwnNotThePackageOwn names the dimension the fix's third
// guard reads. A //go:build constraint raises one file above the module's go
// directive, so two files of the same package can disagree about whether any is
// a legal spelling — and a guard that consults only the package answers for the
// wrong file in a package that mixes them.
func TestTheVersionIsTheFileOwnNotThePackageOwn(t *testing.T) {
	t.Parallel()
	const plain = "package p\n\ntype held interface{}\n"
	const raised = "//go:build go1.21\n\npackage p\n\ntype raisedHeld interface{}\n"

	pass, its := checkedAt(t, "go1.17", plain, raised)
	require.Equal(t, "go1.17", pass.Pkg.GoVersion(), "the package sits below the boundary")

	assert.Nil(t, fixes(pass, its[0]), "the file taking the package's version cannot spell any")
	assert.Len(t, fixes(pass, its[1]), 1, "the file its own constraint raised above go1.18 can")
}
