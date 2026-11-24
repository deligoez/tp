package engine

import (
	"os"
	"testing"
)

// TestMain clears every variable a tp run unit carries. A unit runs this
// suite from its own shell, and an inherited TP_FILE made Discover resolve the
// run's task file instead of a test's fixture, and write into it.
func TestMain(m *testing.M) {
	for _, name := range RunEnvVars {
		_ = os.Unsetenv(name)
	}
	os.Exit(m.Run())
}
