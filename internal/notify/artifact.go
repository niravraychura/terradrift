package notify

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/niravraychura/terradrift/internal/redact"
)

// ArtifactUploader uploads a report to a presigned HTTPS URL.
type ArtifactUploader struct {
	URL    string
	Client HTTPDoer
}

// Upload writes content to the configured artifact URL without following redirects.
func (uploader ArtifactUploader) Upload(ctx context.Context, content []byte, contentType string) error {
	artifactURL, err := validateGenericWebhookURL(uploader.URL)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, artifactURL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create artifact upload request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("User-Agent", "terradrift")
	client := uploader.Client
	if client == nil {
		client = secureWebhookClient()
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("upload artifact to %s: %w", redact.String(artifactURL), err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("upload artifact to %s: unexpected status %s", redact.String(artifactURL), response.Status)
	}
	return nil
}
