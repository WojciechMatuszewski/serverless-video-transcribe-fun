package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/transcribe/types"
)

func main() {
	lambda.Start(NewHandler())
}

type Input struct {
	ExecutionName string `json:"executionName"`
}

type Handler func(ctx context.Context, input Input) (bool, error)

func NewHandler() Handler {
	return func(ctx context.Context, input Input) (bool, error) {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			panic(err)
		}

		client := transcribe.NewFromConfig(cfg)
		paginator := transcribe.NewListTranscriptionJobsPaginator(client, &transcribe.ListTranscriptionJobsInput{
			JobNameContains: aws.String(input.ExecutionName),
		})

		fmt.Println("Fetching summaries for ExecutionName", input.ExecutionName)

		var summaries []types.TranscriptionJobSummary
		for paginator.HasMorePages() {
			jobs, err := paginator.NextPage(ctx)
			if err != nil {
				panic(err)
			}

			summaries = append(summaries, jobs.TranscriptionJobSummaries...)
		}

		fmt.Println("Fetched", len(summaries), "summaries")

		if len(summaries) == 0 {
			panic("No jobs!")
		}

		isDone := true
		for _, summary := range summaries {
			if summary.TranscriptionJobStatus != types.TranscriptionJobStatusCompleted {
				isDone = false
				break
			}
		}

		fmt.Println("Are all jobs finished?", isDone)

		return isDone, nil
	}
}
