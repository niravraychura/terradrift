// Package redact provides helpers for removing sensitive values from logs,
// errors, notifications, and other user-visible output.
package redact

import (
	"net/url"
	"regexp"
	"strings"
)

const replacement = "[REDACTED]"

var sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(authorization|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret|password|secret|token)(\s*[=:]\s*)([^\s,;&]+)`)

// String removes common secret patterns from a string while preserving enough
// surrounding context to keep diagnostic messages useful.
func String(value string) string {
	redacted := redactSensitiveAssignments(value)
	redacted = redactURLs(redacted)
	return redacted
}

func redactSensitiveAssignments(value string) string {
	return sensitiveAssignmentPattern.ReplaceAllString(value, `${1}${2}`+replacement)
}

func redactURLs(value string) string {
	fields := strings.Fields(value)
	for i, field := range fields {
		trimmed := strings.Trim(field, `"'(),;`)
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}

		if shouldRedactURL(parsed) {
			redactedURL := *parsed
			redactedURL.User = nil
			query := redactedURL.Query()
			for key := range query {
				if isSensitiveKey(key) {
					query.Set(key, replacement)
				}
			}
			redactedURL.RawQuery = query.Encode()
			if isSensitiveHost(redactedURL.Host) {
				redactedURL.Path = "/" + replacement
				redactedURL.RawQuery = ""
			}
			fields[i] = strings.Replace(field, trimmed, redactedURL.String(), 1)
		}
	}
	return strings.Join(fields, " ")
}

func shouldRedactURL(parsed *url.URL) bool {
	if parsed.User != nil || isSensitiveHost(parsed.Host) {
		return true
	}
	for key := range parsed.Query() {
		if isSensitiveKey(key) {
			return true
		}
	}
	return false
}

func isSensitiveHost(host string) bool {
	normalizedHost := strings.ToLower(host)
	return strings.Contains(normalizedHost, "hooks.slack.com") || strings.Contains(normalizedHost, "webhook.office.com")
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "password") || strings.Contains(key, "key")
}
