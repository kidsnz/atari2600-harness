// Package version is the single source of truth for the harness version.
//
// It exists because the version had drifted between CHANGELOG.md and the MCP
// server's serverInfo more than once: two hand-maintained copies of the same
// number will eventually disagree, and a tool that stamps a wrong version into a
// provenance block makes every artifact it produced unreliable to trace back.
package version

// Harness is the current harness version (SemVer). Bump it in the same commit as
// the CHANGELOG entry and the git tag.
const Harness = "1.113.0"
