package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

const bucketName = "serverlesstranscribestack2-databucketd8691f4e-zvhxrv0mmib1"

func Test_Main(t *testing.T) {
	os.Setenv("BUCKET_NAME", bucketName)

	h := NewHandler()

	out, err := h(context.Background(), events.APIGatewayV2HTTPRequest{})
	if err != nil {
		t.Fatal(err)
	}

	type Output struct {
		URL string `json:"url,omitempty"`
	}

	var output Output
	err = json.Unmarshal([]byte(out.Body), &output)

	if err != nil {
		t.Fatal(err)
	}

	if output.URL == "" {
		t.Fatal(errors.New("output URL is empty"))
	}

}
