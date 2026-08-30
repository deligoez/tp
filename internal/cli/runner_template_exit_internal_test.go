package cli

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// §3.2.1's two template failures — a name that is not a built-in template, and
// a placeholder the driver cannot resolve — are USAGE errors (exit 2), the same
// classification §3.2's shape errors take and for the same reason: the
// configuration does not say what to spawn, so no retry and no lock wait can
// change the answer. Asserted through dispatchError because Execute ends in
// os.Exit.
func TestDispatchError_RunnerTemplateIsUsage(t *testing.T) {
	unknownName := func() error {
		_, err := engine.ResolveUnitRunner(json.RawMessage(`"cladue"`), engine.TemplateValues{Kind: engine.UnitImplement})
		return err
	}
	badPlaceholder := func() error {
		_, err := engine.ResolveUnitRunner(
			json.RawMessage(`{"cmd":"/bin/echo","args":["{model}"]}`), engine.TemplateValues{Kind: engine.UnitAuditRole})
		return err
	}

	for name, produce := range map[string]func() error{
		"unknown template name":  unknownName,
		"unresolved placeholder": badPlaceholder,
	} {
		err := produce()
		require.Error(t, err, "%s is a usage error", name)

		code, msg, hint := dispatchError(err)
		assert.Equal(t, ExitUsage, code, "%s exits 2, not %d", name, code)
		assert.Equal(t, err.Error(), msg, "the message names the place at fault")
		assert.NotEqual(t, runtimeFailureHint, hint,
			"%s carries its own hint, not the exit-1 default", name)

		var tmplErr *engine.RunnerTemplateError
		require.ErrorAs(t, err, &tmplErr)
		assert.Equal(t, tmplErr.Hint(), hint)
	}
}

// The classification survives wrapping: the driver that resolves a runner sits
// several frames below the dispatcher, and %w is how its error gets there.
func TestDispatchError_RunnerTemplateSurvivesWrapping(t *testing.T) {
	_, err := engine.BuiltinRunner("gpt", engine.TemplateValues{Kind: engine.UnitImplement})
	require.Error(t, err)

	code, _, hint := dispatchError(fmt.Errorf("resolving the runner for implement: %w", err))
	assert.Equal(t, ExitUsage, code, "a wrapped template error is still a usage error")
	assert.Contains(t, hint, "opencode", "the hint survives the wrapping too")
}
