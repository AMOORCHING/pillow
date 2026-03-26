package tts

import (
	"bytes"
	"io"
	"os"
)

func newBytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

func createTempFile(pattern string) (*os.File, error) {
	return os.CreateTemp("", pattern)
}

func removeFile(f *os.File) {
	os.Remove(f.Name())
}
