package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoadRecent(b *testing.B) {
	dir := b.TempDir()
	const files = 200
	for i := 0; i < files; i++ {
		name := fmt.Sprintf("%03d-no_drift.json", i)
		payload := []byte(`{"status":"no_drift","directory":"terraform/prod","total_resources_checked":10}`)
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o600); err != nil {
			b.Fatalf("write history fixture: %v", err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries, err := LoadRecent(dir, 10)
		if err != nil {
			b.Fatalf("LoadRecent: %v", err)
		}
		if len(entries) != 10 {
			b.Fatalf("expected 10 entries, got %d", len(entries))
		}
	}
}
