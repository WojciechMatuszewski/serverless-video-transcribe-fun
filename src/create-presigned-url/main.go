package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	lambda.Start(NewHandler())
}

type Response struct {
	URL string `json:"url"`
}

type Handler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

func NewHandler() Handler {
	return func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			panic(err)
		}

		client := s3.NewFromConfig(cfg)
		psClient := s3.NewPresignClient(client)

		bucket := os.Getenv("BUCKET_NAME")
		if bucket == "" {
			panic("No bucket")
		}

		out, err := psClient.PresignPutObject(
			ctx,
			&s3.PutObjectInput{
				Key:    aws.String("input/file.mp4"),
				Bucket: aws.String(bucket),
			},
			s3.WithPresignExpires(time.Minute*5),
		)

		fmt.Println(out.SignedHeader.Get("expires"))

		if err != nil {
			panic(err)
		}

		response := Response{URL: out.URL}
		buf, err := json.Marshal(&response)
		if err != nil {
			panic(err)
		}

		fmt.Println(out.URL)

		return events.APIGatewayV2HTTPResponse{
			Body:       string(buf),
			StatusCode: http.StatusOK,
		}, nil
	}
}
