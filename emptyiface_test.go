package emptyiface_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/analysis/analysistest"

	emptyiface "github.com/gomatic/yze-go-emptyiface"
)

func TestEmptyInterfaceIsReportedAndFixed(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), emptyiface.Analyzer, "a")
}

func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, emptyiface.Registration.Validate())
	assert.Equal(t, "yze/emptyiface", emptyiface.Registration.RuleID())
	assert.Same(t, emptyiface.Analyzer, emptyiface.Registration.Analyzer)
}
