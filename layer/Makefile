.PHONY: build clean

clean:
	rm -rf tmp
	rm -rf bin

build-binaries:
	mkdir -p tmp
	mkdir -p bin
	cd tmp && curl https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz | tar x
	mv tmp/ffmpeg*/ffmpeg tmp/ffmpeg*/ffprobe bin

build: clean build-binaries
