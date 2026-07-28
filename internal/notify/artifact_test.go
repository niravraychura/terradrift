package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestArtifactUploaderUploadsJSON(t *testing.T) {
	uploader := ArtifactUploader{
		URL: "https://artifacts.example.com/report?token=secret-value",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodPut || request.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("unexpected artifact request: %s %#v", request.Method, request.Header)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil || string(body) != `{"status":"no_drift"}` {
				t.Fatalf("unexpected artifact body: %q, %v", body, err)
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}
	if err := uploader.Upload(context.Background(), []byte(`{"status":"no_drift"}`), "application/json"); err != nil {
		t.Fatalf("upload artifact: %v", err)
	}
}
