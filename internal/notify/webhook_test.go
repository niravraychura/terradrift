package notify

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestWebhookNotifierSendsRedactedPayload(t *testing.T) {
	var body string
	notifier := WebhookNotifier{
		WebhookURL: "https://alerts.example.com/terradrift?token=secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected JSON content type, got %q", got)
			}
			data, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			body = string(data)
			return &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{
		Status:                report.ScanStatusDriftDetected,
		PlanMode:              "refresh-only",
		Directory:             "/secret/local/path",
		TotalResourcesChecked: 8,
		TotalChangedResources: 3,
	})
	if err != nil {
		t.Fatalf("expected webhook notification to succeed: %v", err)
	}
	if strings.Contains(body, "/secret/local/path") || strings.Contains(body, "secret-value") {
		t.Fatalf("expected webhook body to avoid path and URL secrets, got %q", body)
	}
	var payload webhookPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("expected webhook JSON, got %v: %q", err, body)
	}
	if payload.Status != report.ScanStatusDriftDetected || payload.PlanMode != "refresh-only" || payload.TotalChangedResources != 3 || payload.TotalResourcesChecked != 8 {
		t.Fatalf("unexpected webhook payload: %#v", payload)
	}
}

func TestWebhookNotifierRejectsUnsafeURLs(t *testing.T) {
	for _, webhookURL := range []string{
		"",
		"http://alerts.example.com/terradrift",
		"https://user:pass@alerts.example.com/terradrift",
		"https://localhost/terradrift",
		"https://127.0.0.1/terradrift",
		"https://10.0.0.1/terradrift",
		"https://169.254.169.254/latest/meta-data",
	} {
		t.Run(webhookURL, func(t *testing.T) {
			err := WebhookNotifier{WebhookURL: webhookURL}.Notify(context.Background(), report.DriftReport{})
			if err == nil {
				t.Fatal("expected unsafe webhook URL to be rejected")
			}
		})
	}
}

func TestBlockedWebhookIPs(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "198.18.0.1", "224.0.0.1", "::1", "fc00::1"} {
		t.Run(value, func(t *testing.T) {
			if !isBlockedWebhookIP(net.ParseIP(value)) {
				t.Fatalf("expected %s to be blocked", value)
			}
		})
	}
}

func TestWebhookNotifierDoesNotExposeMalformedURL(t *testing.T) {
	secret := "secret-value"
	err := WebhookNotifier{WebhookURL: "https://example.com/?token=" + secret + "%zz"}.Notify(context.Background(), report.DriftReport{})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("expected safe malformed URL error, got %v", err)
	}
}

func TestSecureWebhookClientDoesNotFollowRedirects(t *testing.T) {
	if err := secureWebhookClient().CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("expected redirects to be rejected, got %v", err)
	}
}

func TestSecureWebhookClientFromCA(t *testing.T) {
	client, err := secureWebhookClientFromCA("")
	if err != nil || client == nil {
		t.Fatalf("expected empty CA path to succeed: %v", err)
	}
	if client.Timeout != webhookHTTPTimeout {
		t.Fatalf("timeout = %v, want %v", client.Timeout, webhookHTTPTimeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.TLSHandshakeTimeout != webhookTLSHandshakeTimeout || transport.ResponseHeaderTimeout != webhookResponseHeaderTimeout {
		t.Fatalf("unexpected transport timeouts: tls=%v header=%v", transport.TLSHandshakeTimeout, transport.ResponseHeaderTimeout)
	}
	if _, err := secureWebhookClientFromCA(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("expected missing CA file to fail")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write bad CA: %v", err)
	}
	if _, err := secureWebhookClientFromCA(bad); err == nil {
		t.Fatal("expected invalid CA PEM to fail")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "terradrift-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	withCA, err := secureWebhookClientFromCA(path)
	if err != nil || withCA == nil {
		t.Fatalf("expected CA client: %v", err)
	}
	transport, ok = withCA.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("expected TLS RootCAs to be configured")
	}
}

func TestWebhookNotifierRedactsURLInErrors(t *testing.T) {
	notifier := WebhookNotifier{
		WebhookURL: "https://alerts.example.com/terradrift?token=secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Body: io.NopCloser(strings.NewReader("no"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{})
	if err == nil {
		t.Fatal("expected webhook status error")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected webhook URL query secret to be redacted from error, got %v", err)
	}
}
