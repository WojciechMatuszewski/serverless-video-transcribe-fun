package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	lambda.Start(NewHandler())
}

type Input struct {
	OriginFilePath     string    `json:"originFilePath,omitempty"`
	OutputFileName     string    `json:"outputFileName,omitempty"`
	EFSOutputDirectory string    `json:"efsOutputDirectory,omitempty"`
	S3OutputDirectory  string    `json:"s3OutputDirectory,omitempty"`
	Chunk              []float64 `json:"chunk,omitempty"`
}

type Handler func(ctx context.Context, input Input) error

func NewHandler() Handler {
	return func(ctx context.Context, input Input) error {
		fPath := fmt.Sprintf("%v/%v", input.EFSOutputDirectory, input.OutputFileName)
		fd, err := os.Open(fPath)
		if err != nil {
			return err
		}
		defer fd.Close()

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return err
		}
		s3Service := s3.NewFromConfig(cfg)
		_, err = s3Service.PutObject(
			ctx,
			&s3.PutObjectInput{
				Bucket: aws.String(os.Getenv("BUCKET_NAME")),
				Key:    aws.String(fmt.Sprintf("%v/%v", input.S3OutputDirectory, input.OutputFileName)),
				Body:   fd,
			},
		)
		if err != nil {
			return err
		}

		return nil
	}
}
