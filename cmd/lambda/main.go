package main

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/Perry2004/GP-News/internal/app"
)

func main() {
	lambda.Start(func(ctx context.Context, event json.RawMessage) (app.Result, error) {
		return app.Run(ctx)
	})
}
