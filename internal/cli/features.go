package cli

import "os"

// addonsEnvVar gates the add-on command surface.
const addonsEnvVar = "DWELLIR_ADDONS"

// addonsEnabled reports whether the add-on commands are registered for this run.
//
// Merging to main tags and publishes a release, so the add-on work has to land
// inert rather than wait for a merge window. Registration is skipped when the
// gate is off, which means a released binary has no `addons` command at all:
// absent from --help and from shell completions, and it cannot be run. That is
// the reason for a runtime gate instead of `Hidden: true`, which would still
// ship a working command.
//
// Telemetry is not silent, though: an attempt to run an unregistered command
// still reports the generic failure event with `unknown_command` set, the same
// as any typo. No add-on command event is ever emitted.
//
// A build tag was rejected for the opposite reason: it would hide the code from
// `go test ./...` and from golangci-lint, so regressions would rot unnoticed.
//
// The gate is deleted in the final phase, and that phase's VERSION bump is the
// real release.
func addonsEnabled() bool {
	return parseBoolFlag(os.Getenv(addonsEnvVar))
}
