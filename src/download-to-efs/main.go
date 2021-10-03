package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	rootPath := "/mnt/videos"

	keyParts := strings.Split(record.S3.Object.Key, "/")
	keyDir := strings.Join(keyParts[:len(keyParts)-1], "/")

	downloadDirLocation := fmt.Sprintf("%v/%v", rootPath, keyDir)
	fmt.Println("Creating directory for the file", downloadDirLocation)
	err := os.MkdirAll(downloadDirLocation, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	downloadToLocation := fmt.Sprintf("%v/%v", rootPath, record.S3.Object.Key)
	fmt.Println("Downloading file to", downloadToLocation)
	fPath := filepath.Join(downloadToLocation)
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
