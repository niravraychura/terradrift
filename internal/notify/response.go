package notify

import "io"

const maxResponseBodyBytes = 32 * 1024

func closeResponseBody(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxResponseBodyBytes))
	return body.Close()
}
