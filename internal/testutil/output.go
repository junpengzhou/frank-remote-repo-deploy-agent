package testutil

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func CaptureStdout(t testing.TB, fn func()) string {
	t.Helper()
	return captureOutput(t, &os.Stdout, fn)
}

func CaptureStderr(t testing.TB, fn func()) string {
	t.Helper()
	return captureOutput(t, &os.Stderr, fn)
}

func captureOutput(t testing.TB, target **os.File, fn func()) string {
	t.Helper()
	original := *target
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	*target = write
	defer func() {
		*target = original
	}()

	fn()

	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, read); err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
