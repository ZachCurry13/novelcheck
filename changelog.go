// Package novelcheck exposes repository-level assets to the application.
package novelcheck

import _ "embed"

// Changelog is CHANGELOG.md, bundled so release notes are readable offline
// and for the exact version that is running.
//
//go:embed CHANGELOG.md
var Changelog string
