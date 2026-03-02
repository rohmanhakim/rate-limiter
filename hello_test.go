package ratelimiter

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestHello_SayHello(t *testing.T) {
	h := &Hello{}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	h.SayHello()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	expected := "Hello, world!\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}
