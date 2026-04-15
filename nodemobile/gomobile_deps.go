//go:build !android

package nodemobile

// 本文件承载 MetricsNode `nodemobile` 桥接中与 `gomobile_deps` 相关的逻辑。

import (
	// gomobile runs `gobind` on the host during `gomobile bind`, and gobind
	// locates `golang.org/x/mobile/bind` via the current module graph.
	_ "golang.org/x/mobile/bind"
)
