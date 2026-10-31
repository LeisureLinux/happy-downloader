package profiles

import "log/slog"

// logWarn is a narrow seam so that browserinstalled.go can stay free of the
// logging import.
func logWarn(message string, args ...any) {
	slog.Warn(message, args...)
}
