package emptyiface_test

import (
	"os"
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	emptyiface "github.com/gomatic/yze-go-emptyiface"
)

// packagePattern is a package the fixture tree holds, named the way a go/analysis
// driver names one.
type packagePattern string

// fixturePackages are every fixture package, chosen to vary the three input
// dimensions a corpus of one directory holds constant — the file PATH (nested),
// the file CLASS (a _test.go inside nested), and the PACKAGE CLAUSE (mainpkg) —
// so a disjunct keyed on any of them fails a case rather than a coverage number.
var fixturePackages = []packagePattern{"a", "spelling", "nested/deep", "mainpkg"}

// patterns renders the fixture packages as the driver's argument list.
func patterns() []string {
	out := make([]string, 0, len(fixturePackages))
	for _, p := range fixturePackages {
		out = append(out, string(p))
	}
	return out
}

func TestEmptyInterfaceIsReportedAndFixed(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), emptyiface.Analyzer, patterns()...)
}

// coveredText is the source text a diagnostic's span selects.
type coveredText string

// spanned reports the source text between a diagnostic's Pos and End, which is
// what an editor highlights and what a caller uses to locate the construct. It
// is read back out of the file the position belongs to, so a span that has been
// collapsed or widened yields different text rather than the same text.
func spanned(t *testing.T, pass *analysis.Pass, diag analysis.Diagnostic) coveredText {
	t.Helper()
	file := pass.Fset.File(diag.Pos)
	require.NotNil(t, file, "every diagnostic position belongs to a file")
	source, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	return coveredText(source[file.Offset(diag.Pos):file.Offset(diag.End)])
}

// TestDiagnosticIsTheContractTheDocCommentStates asserts the three things the
// corpus Finding {rule, path, line} cannot carry and the golden files do not
// read: the exact message, the exact reported extent, and the exact fix title.
// Each is user-facing — the sentence a reader sees, the region an editor
// highlights, the entry a quick-fix menu offers — and none of them is pinned by
// a want-regex that matches any message sharing eleven characters.
func TestDiagnosticIsTheContractTheDocCommentStates(t *testing.T) {
	results := analysistest.Run(t, analysistest.TestData(), emptyiface.Analyzer, "spelling")
	require.Len(t, results, 1)
	diagnostics := results[0].Diagnostics
	require.NotEmpty(t, diagnostics, "the spelling fixture reports")

	for _, diag := range diagnostics {
		assert.Equal(t, "prefer any to the empty interface{}", diag.Message,
			"the whole sentence is the contract, not its first eleven characters")
		assert.Empty(t, diag.Category, "the rule carries no diagnostic sub-category")
		require.Len(t, diag.SuggestedFixes, 1, "every fixture spelling is rewritable, so every one carries the fix")
		assert.Equal(t, "replace interface{} with any", diag.SuggestedFixes[0].Message,
			"the title is what a quick-fix menu offers and analysistest never compares it")
		require.Len(t, diag.SuggestedFixes[0].TextEdits, 1)
		edit := diag.SuggestedFixes[0].TextEdits[0]
		assert.Equal(t, "any", string(edit.NewText))
		assert.Equal(t, diag.Pos, edit.Pos, "the edit replaces exactly the reported extent")
		assert.Equal(t, diag.End, edit.End, "the edit replaces exactly the reported extent")
	}
}

// TestReportedExtentIsTheWholeInterfaceType pins the span against the source it
// must select. A span collapsed to its start, or one stopping short of the
// closing brace, still satisfies every golden file — the applied bytes are the
// same only because the edit follows the span — so the extent is asserted here
// against the text it covers.
func TestReportedExtentIsTheWholeInterfaceType(t *testing.T) {
	results := analysistest.Run(t, analysistest.TestData(), emptyiface.Analyzer, "spelling")
	require.Len(t, results, 1)

	covered := make([]coveredText, 0, len(results[0].Diagnostics))
	for _, diag := range results[0].Diagnostics {
		covered = append(covered, spanned(t, results[0].Pass, diag))
	}

	assert.Equal(t, []coveredText{
		"interface{}",
		"interface{ any }",
		"interface{ Short }",
	}, covered, "the reported extent is the whole interface type, in every spelling")
}

// TestRegistrationIsWellFormed pins every field of the Registration, not only the
// two Validate checks. Categories decide which rules a --category run reaches and
// URL is stamped onto every Diagnostic the analyzer emits, so both are part of
// what the rule MEANS to a user and neither is asserted by Validate.
func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, emptyiface.Registration.Validate())
	assert.Equal(t, "yze/emptyiface", emptyiface.Registration.RuleID())
	assert.Same(t, emptyiface.Analyzer, emptyiface.Registration.Analyzer)
	assert.Equal(t, goyze.AnalyzerName("emptyiface"), emptyiface.Registration.Name)
	assert.Equal(t, []goyze.Category{"modern-go", "types"}, emptyiface.Registration.Categories,
		"types is what namedtypes and anonstruct are selected by, and this rule is the same axis")
	assert.Equal(t, goyze.HelpURL("https://docs.gomatic.dev/yze/emptyiface"), emptyiface.Registration.URL)
	assert.Equal(t, goyze.TestScopeAll, emptyiface.Registration.TestScope,
		"a spelling rule has no test-side counterpart, so it reports in every file class")
}
