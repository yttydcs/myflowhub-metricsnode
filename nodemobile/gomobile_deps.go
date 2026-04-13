//go:build !android

package nodemobile

// Context: This file belongs to the MetricsNode application layer around gomobile_deps.

import (
	// gomobile runs `gobind` on the host during `gomobile bind`, and gobind
	// locates `golang.org/x/mobile/bind` via the current module graph.
	_ "golang.org/x/mobile/bind"
)
