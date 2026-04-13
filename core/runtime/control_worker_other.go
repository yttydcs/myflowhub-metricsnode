//go:build !windows

package runtime

// Context: This file belongs to the MetricsNode application layer around control_worker_other.

import "context"

func (r *Runtime) startControlWorker(_ context.Context) {}
