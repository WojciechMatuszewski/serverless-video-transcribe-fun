package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(NewHandler())
}

type Input struct {
	FileName string `json:"fileName,omitempty"`
	FilePath string `json:"filePath,omitempty"`
}

type Handler func(ctx context.Context, input Input) ([][]float64, error)

func NewHandler() Handler {
	return func(ctx context.Context, input Input) ([][]float64, error) {

		fmt.Println("Reading movie duration")
		d, err := getMovieDuration(input.FilePath)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}

		fmt.Println("Duration", d)

		fmt.Println("Splitting duration into 5 seconds chunks")
		chunks, err := durationToChunks(d, 5.0)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}

		fmt.Println("Chunks", chunks)

		return chunks, nil
	}
}

func getMovieDuration(fp string) (string, error) {
	cmd := exec.Command(
		"/opt/ffprobe", "-show_entries",
		"format=duration", "-of",
		"default=noprint_wrappers=1:nokey=1",
		"-sexagesimal", fp,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.Trim(string(out), "\n"), nil
}

func durationToChunks(duration string, chunkDurationSeconds float64) ([][]float64, error) {

	dParts := strings.Split(duration, ":")
	if len(dParts) != 3 {
		return [][]float64{}, errors.New("malformed duration format")
	}

	h, err := strconv.ParseFloat(dParts[0], 64)
	if err != nil {
		return [][]float64{}, err
	}
	hs := 0.0
	if h != 0 {
		hs = h * float64(60) * float64(60)
	}

	m, err := strconv.ParseFloat(dParts[1], 64)
	if err != nil {
		return [][]float64{}, err
	}

	ms := 0.0
	if m != 0 {
		ms = m * 60.0
	}

	sParts := strings.Split(dParts[2], ".")
	if len(sParts) != 2 {
		return [][]float64{}, errors.New("malformed seconds format")
	}

	s, err := strconv.ParseFloat(sParts[0], 64)
	if err != nil {
		return [][]float64{}, err
	}
	mss := sParts[1]

	seconds := hs + ms + s

	start := 0.0
	end, err := strconv.ParseFloat(fmt.Sprintf("%v.%s", seconds, mss), 64)
	if err != nil {
		return [][]float64{}, err
	}

	chunks := [][]float64{}
	for {

		if seconds-(start+2*chunkDurationSeconds) <= 0 {
			chunks = append(chunks, []float64{start, end})
			break
		}

		chunks = append(chunks, []float64{start, start + chunkDurationSeconds})
		start += chunkDurationSeconds
	}

	return chunks, err
}
