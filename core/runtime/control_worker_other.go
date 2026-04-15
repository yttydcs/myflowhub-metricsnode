//go:build !windows

package runtime

// 本文件承载 MetricsNode 应用层中与 `control_worker_other` 相关的逻辑。

import "context"

func (r *Runtime) startControlWorker(_ context.Context) {}
