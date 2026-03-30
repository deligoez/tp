package cli

// Exit codes per spec Section 12.1.
const (
	ExitSuccess    = 0 // Success
	ExitValidation = 1 // Validation error (structure, atomicity in strict mode, cycles, closure verification)
	ExitUsage      = 2 // Usage error (wrong arguments, missing required flags)
	ExitFile       = 3 // File error (file not found, permission denied, invalid JSON)
	ExitState      = 4 // State error (wrong status transition, blocked task, no ready tasks for --first)
)
