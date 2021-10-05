package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(NewHandler())
}

type Input struct {
	OriginFilePath     string    `json:"originFilePath,omitempty"`
	OutputFileName     string    `json:"outputFileName,omitempty"`
	EFSOutputDirectory string    `json:"efsOutputDirectory,omitempty"`
	Chunk              []float64 `json:"chunk,omitempty"`
}

type Handler func(ctx context.Context, input Input) error

func NewHandler() Handler {
	return func(ctx context.Context, input Input) error {
		fmt.Println("Creating output directory")
		err := os.MkdirAll(input.EFSOutputDirectory, os.ModePerm)
		if err != nil {
			return err
		}

		cmd := exec.Command(
			"/opt/ffmpeg",
			"-i", input.OriginFilePath,
			"-ss", fmt.Sprintf("%.6f", input.Chunk[0]),
			"-t", fmt.Sprintf("%.6f", input.Chunk[1]),
			"-c", "copy", fmt.Sprintf("%v/%v", input.EFSOutputDirectory, input.OutputFileName),
		)
		fmt.Println("Invoking", cmd.String())

		out, err := cmd.CombinedOutput()
		fmt.Println("Command output", string(out))
		if err != nil {
			return err
		}

		return nil
	}
}
