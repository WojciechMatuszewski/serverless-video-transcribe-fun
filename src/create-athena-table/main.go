package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
)

func main() {

}

type Handler func(ctx context.Context, event cfn.Event) (physicalResourceID string, data map[string]interface{}, err error)

func NewHandler() Handler {
	return func(ctx context.Context, event cfn.Event) (physicalResourceID string, data map[string]interface{}, err error) {
		if event.RequestType != cfn.RequestCreate {
			return
		}

		bucketName, ok := event.ResourceProperties["bucketName"].(string)
		if !ok {
			fmt.Println("bucketName param not found", event.ResourceProperties)
			err = errors.New("not found")
			return
		}

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}

		aClient := athena.NewFromConfig(cfg)

		q := fmt.Sprintf("%s\nLOCATION 's3://%s/subtitles'", query, bucketName)
		_, err = aClient.StartQueryExecution(
			ctx,
			&athena.StartQueryExecutionInput{
				QueryString: aws.String(q),
			},
		)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}

		return
	}
}

const query = `
	CREATE EXTERNAL TABLE subtitles (
		jobName string,
		results struct<transcripts:array<struct<transcript:string>>>
	)
	ROW FORMAT SERDE 'org.openx.data.jsonserde.JsonSerDe'
`
