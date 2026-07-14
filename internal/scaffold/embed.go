package scaffold

import "embed"

// assets contains the versioned starter snapshot and composable overlays.
//
//go:embed all:_assets
var assets embed.FS
