package emptyiface

// White-box test for the file lookup the fix anchors on.

import (
	"go/ast"
	"go/parser"
	"go/token"
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
