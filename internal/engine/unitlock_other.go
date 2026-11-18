//go:build !unix

package engine

import "os"

// lockUnitFile takes no lock where a descriptor cannot be handed to a child.
func lockUnitFile(string) (*os.File, error) { return nil, nil }

// processAlive is never asked where no unit lock is taken; it says no.
func processAlive(int) bool { return false }

// unitLockHeld is never true where no unit lock is taken.
func unitLockHeld(string) bool { return false }
