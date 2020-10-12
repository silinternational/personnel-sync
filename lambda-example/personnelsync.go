package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	personnel_sync "github.com/silinternational/personnel-sync/v5"
)

type LambdaConfig struct {
	ConfigPath string
}

func main() {
	lambda.Start(handler)
}

func handler(lambdaConfig LambdaConfig) error {
	return personnel_sync.RunSync(lambdaConfig.ConfigPath)
}
