package redact

import (
	"strings"
	"testing"
)

func BenchmarkString(b *testing.B) {
	var builder strings.Builder
	builder.Grow(64 << 10)
	for i := 0; i < 200; i++ {
		builder.WriteString("harmless=value token=secret-token password:hunter2 ")
		builder.WriteString("https://hooks.slack.com/services/T000/B000/secret-value ")
		builder.WriteString("https://example.com/path?api_key=abc123&key_id=keep ")
	}
	input := builder.String()
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = String(input)
	}
}
