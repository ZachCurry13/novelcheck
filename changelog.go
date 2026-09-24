// Package novelcheck exposes repository-level documents bundled into the app.
package novelcheck

import _ "embed"

// Changelog is CHANGELOG.md, bundled so release notes are readable offline
// and for the exact version that is running.
//
//go:embed CHANGELOG.md
var Changelog string

// ProviderGuide is docs/AI_PROVIDERS.md, shown in Admin → LLM Analysis Engine
// as step-by-step setup instructions for each AI provider.
//
//go:embed docs/AI_PROVIDERS.md
var ProviderGuide string

// RemoteAccessGuide is docs/REMOTE_ACCESS.md, shown in Admin → Remote access.
//
//go:embed docs/REMOTE_ACCESS.md
var RemoteAccessGuide string

// CalibreServerGuide is docs/CALIBRE_SERVER.md, shown in Admin → Calibre
// Library for setting up one-click removal.
//
//go:embed docs/CALIBRE_SERVER.md
var CalibreServerGuide string
