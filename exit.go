package main

// Exit codes. They are distinct so a script or an agent can tell a crash from
// a refusal from a failed health check, instead of branching on stderr text.
const (
	// exitOK means the requested work finished.
	exitOK = 0
	// exitError means something failed at runtime: Docker unreachable, a
	// command that returned non-zero, an unreadable registry.
	exitError = 1
	// exitUsage means the invocation itself was wrong: an unknown flag, a bad
	// argument, an out-of-range suffix. Matches the code Go's flag package
	// already uses for parse errors.
	exitUsage = 2
	// exitAborted means sailinit stopped without doing the work because it
	// needed a human: the user declined a prompt, or there was no terminal to
	// ask on and --yes was not given.
	exitAborted = 3
	// exitUnhealthy means --doctor ran fine but at least one check FAILed.
	// Distinct from exitError so a gate can tell "doctor found problems" from
	// "doctor itself broke".
	exitUnhealthy = 4
)
