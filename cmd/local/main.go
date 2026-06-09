package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Perry2004/GP-News/internal/app"
)

func main() {
	res, err := app.Run(context.Background())
	if err != nil {
		slog.Error("GP-News failed", "error", err)
		os.Exit(1)
	}
	slog.Info("GP-News completed", "result", res)
}
