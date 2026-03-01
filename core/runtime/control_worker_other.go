//go:build !windows

package runtime

import "context"

func (r *Runtime) startControlWorker(_ context.Context) {}

