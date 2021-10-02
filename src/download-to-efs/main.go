package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, event events.S3Event) {
	record := event.Records[0]

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fPath := filepath.Join("/mnt/videos", record.S3.Object.Key)
	fd, err := os.Create(fPath)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer fd.Close()

	dManager := manager.NewDownloader(s3.NewFromConfig(cfg))
	_, err = dManager.Download(
		ctx,
		fd,
		&s3.GetObjectInput{
			Bucket: &record.S3.Bucket.Name,
			Key:    &record.S3.Object.Key,
		},
	)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fmt.Println("OK")
}
