package notify

import (
	"io"
	"strings"
	"testing"
)

func TestCloseResponseBodyBoundsDrain(t *testing.T) {
	reader := strings.NewReader(strings.Repeat("x", maxResponseBodyBytes*2))
	if err := closeResponseBody(io.NopCloser(reader)); err != nil {
		t.Fatalf("close response body: %v", err)
	}
	if reader.Len() != maxResponseBodyBytes {
		t.Fatalf("drained %d bytes, want %d", maxResponseBodyBytes*2-reader.Len(), maxResponseBodyBytes)
	}
}
