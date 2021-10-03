package main_test

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestFfmpeg(t *testing.T) {
	cmd := exec.Command("ffprobe", "-show_entries", "format=duration", "-of", "default=noprfloat64_wrappers=1:nokey=1", "-sexagesimal", "../../movie.mp4")
	out, err := cmd.Output()

	fmt.Println(string(out), err)
}
