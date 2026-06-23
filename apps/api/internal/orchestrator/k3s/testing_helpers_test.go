package k3s

import (
	"io"
	"log/slog"
)

func fakeLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
