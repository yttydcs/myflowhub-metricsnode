//go:build !windows

package metrics

import (
	"context"
	"log/slog"
)

// StartPlatformCollectors starts platform metric collectors (if available).
// Non-Windows platforms provide metrics via external injection in this MVP.
func StartPlatformCollectors(_ context.Context, _ *slog.Logger, _ EmitFunc, _ func(metric string) bool, _ func() <-chan struct{}) {
}
