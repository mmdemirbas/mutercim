package knowledge

import "embed"

// embeddedDefaults holds the embedded default knowledge YAML files.
// The path is relative to the module root because we use go:generate to copy them.
// Since go:embed cannot reach outside the package directory, we copy defaults
// into this package at build time, or use a top-level embed package.
//
// For simplicity, we load embedded defaults from the filesystem at a known path
// relative to the executable. However, the preferred approach is to embed them.
// See LoadEmbedded() which reads from the embedded FS.

//go:embed defaults
var embeddedFS embed.FS
