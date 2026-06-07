package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Perry2004/GP-News/internal/app"
)

func main() {
	if _, err := app.Run(context.Background()); err != nil {
		slog.Error("GP-News failed", "error", err)
		os.Exit(1)
	}
}
