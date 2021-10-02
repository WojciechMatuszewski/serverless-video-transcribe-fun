package main_test

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestFfmpeg(t *testing.T) {
	cmd := exec.Command("ffprobe", "-show_entries", "format=duration", "-of", "default=noprfloat64_wrappers=1:nokey=1", "-sexagesimal", "../../movie.mp4")
	out, err := cmd.Output()

	fmt.Println(string(out), err)
}

func TestDurationToChunks(t *testing.T) {
	chunks, err := durationToChunks("0:03:59.045000", 59.0)
	fmt.Println(chunks, err)
}

func durationToChunks(duration string, chunkDurationSeconds float64) ([][]float64, error) {

	dParts := strings.Split(duration, ":")
	if len(dParts) != 3 {
		return [][]float64{}, errors.New("Malformed duration format")
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
		return [][]float64{}, errors.New("Malformed seconds format")
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
