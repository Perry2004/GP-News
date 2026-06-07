package main

import (
	"github.com/Perry2004/GP-News/internal/app"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(app.HandleLambda)
}
