package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/niravraychura/terradrift/internal/report"
)

const (
	maxAttributeDiffsPerResource = 100
	maxAttributeValueChars       = 200
	redactedValue                = "[REDACTED]"
	absentValue                  = "(absent)"
	unknownValue                 = "(known after apply)"
)

func attributeChangesFor(change terraformChange) []report.AttributeChange {
	before, err := decodeOptionalJSON(change.Before)
	if err != nil {
		return nil
	}
	after, err := decodeOptionalJSON(change.After)
	if err != nil {
		return nil
	}
	beforeSensitive, err := decodeOptionalJSON(change.BeforeSensitive)
	if err != nil {
		beforeSensitive = false
	}
	afterSensitive, err := decodeOptionalJSON(change.AfterSensitive)
	if err != nil {
		afterSensitive = false
	}
	afterUnknown, err := decodeOptionalJSON(change.AfterUnknown)
	if err != nil {
		afterUnknown = false
	}

	beforeLeaves := map[string]leaf{}
	afterLeaves := map[string]leaf{}
	collectLeaves("", before, beforeSensitive, false, beforeLeaves)
	collectLeaves("", after, afterSensitive, false, afterLeaves)
	unknownPaths := map[string]struct{}{}
	collectUnknownPaths("", afterUnknown, unknownPaths)

	paths := make(map[string]struct{}, len(beforeLeaves)+len(afterLeaves)+len(unknownPaths))
	for path := range beforeLeaves {
		paths[path] = struct{}{}
	}
	for path := range afterLeaves {
		paths[path] = struct{}{}
	}
	for path := range unknownPaths {
		paths[path] = struct{}{}
	}

	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	diffs := make([]report.AttributeChange, 0, len(ordered))
	for _, path := range ordered {
		beforeLeaf, hasBefore := beforeLeaves[path]
		afterLeaf, hasAfter := afterLeaves[path]
		_, unknown := unknownPaths[path]
		if !hasBefore && !hasAfter && !unknown {
			continue
		}
		if hasBefore && hasAfter && !unknown && beforeLeaf.value == afterLeaf.value {
			continue
		}

		beforeText := absentValue
		afterText := absentValue
		if hasBefore {
			beforeText = beforeLeaf.value
		}
		if unknown {
			afterText = unknownValue
		} else if hasAfter {
			afterText = afterLeaf.value
		}

		if beforeLeaf.sensitive || afterLeaf.sensitive || sensitivePath(path) {
			if hasBefore {
				beforeText = redactedValue
			} else {
				beforeText = absentValue
			}
			if unknown {
				afterText = unknownValue
			} else if hasAfter {
				afterText = redactedValue
			} else {
				afterText = absentValue
			}
		}
		diffs = append(diffs, report.AttributeChange{Path: path, Before: beforeText, After: afterText})
		if len(diffs) >= maxAttributeDiffsPerResource {
			break
		}
	}
	return diffs
}

type leaf struct {
	value     string
	sensitive bool
}

func decodeOptionalJSON(data json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func collectLeaves(path string, value any, sensitiveMarks any, parentSensitive bool, out map[string]leaf) {
	sensitive := parentSensitive || markSensitive(sensitiveMarks)
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			if path != "" {
				out[path] = leaf{value: formatValue(typed), sensitive: sensitive || sensitivePath(path)}
			}
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := joinPath(path, key)
			collectLeaves(childPath, typed[key], childMark(sensitiveMarks, key), sensitive, out)
		}
	case []any:
		if len(typed) == 0 {
			if path != "" {
				out[path] = leaf{value: formatValue(typed), sensitive: sensitive || sensitivePath(path)}
			}
			return
		}
		for index, item := range typed {
			childPath := joinIndex(path, index)
			collectLeaves(childPath, item, childIndexMark(sensitiveMarks, index), sensitive, out)
		}
	default:
		if path == "" {
			return
		}
		out[path] = leaf{value: formatValue(typed), sensitive: sensitive || sensitivePath(path)}
	}
}

func collectUnknownPaths(path string, value any, out map[string]struct{}) {
	switch typed := value.(type) {
	case bool:
		if typed && path != "" {
			out[path] = struct{}{}
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collectUnknownPaths(joinPath(path, key), typed[key], out)
		}
	case []any:
		for index, item := range typed {
			collectUnknownPaths(joinIndex(path, index), item, out)
		}
	}
}

func markSensitive(marks any) bool {
	flag, ok := marks.(bool)
	return ok && flag
}

func childMark(marks any, key string) any {
	object, ok := marks.(map[string]any)
	if !ok {
		return false
	}
	return object[key]
}

func childIndexMark(marks any, index int) any {
	list, ok := marks.([]any)
	if !ok || index < 0 || index >= len(list) {
		return false
	}
	return list[index]
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	if needsBracket(key) {
		return parent + "[" + strconv.Quote(key) + "]"
	}
	return parent + "." + key
}

func joinIndex(parent string, index int) string {
	if parent == "" {
		return "[" + strconv.Itoa(index) + "]"
	}
	return parent + "[" + strconv.Itoa(index) + "]"
}

func needsBracket(key string) bool {
	if key == "" {
		return true
	}
	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return true
		}
	}
	return false
}

func formatValue(value any) string {
	if value == nil {
		return absentValue
	}
	switch typed := value.(type) {
	case string:
		if len(typed) > maxAttributeValueChars {
			return summarizeBytes(len(typed))
		}
		formatted := strconv.Quote(typed)
		if len(formatted) > maxAttributeValueChars {
			return summarizeBytes(len(typed))
		}
		return formatted
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		text := typed.String()
		if len(text) > maxAttributeValueChars {
			return summarizeBytes(len(text))
		}
		return text
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			text := fmt.Sprint(typed)
			if len(text) > maxAttributeValueChars {
				return summarizeBytes(len(text))
			}
			return text
		}
		if len(encoded) > maxAttributeValueChars {
			return summarizeBytes(len(encoded))
		}
		return string(encoded)
	}
}

func summarizeBytes(size int) string {
	return fmt.Sprintf("[changed, %dB]", size)
}

func sensitivePath(path string) bool {
	lower := strings.ToLower(path)
	for _, marker := range []string{
		"password", "passwd", "secret", "token", "private_key", "access_key",
		"secret_string", "api_key", "apikey", "client_secret", "credentials",
		"connection_string", "database_url", "db_url", "db_password", "db_conn",
		"conn_str", "jdbc", "mongodb_uri", "postgres_url", "mysql_url",
		"user_data", "private_key_pem", "auth_token", "bearer", "oauth",
		"smtp_password", "aws_access_key_id", "session_token", "refresh_token",
		"id_token", "shared_secret", "encryption_key", "master_key", "sas_token",
		"certificate_pem", "ssh_private", "tls_private", "kubeconfig",
		"redis_password", "passphrase", "cloud_credential",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	segment := lower
	if idx := strings.LastIndex(lower, "."); idx >= 0 {
		segment = lower[idx+1:]
	}
	for _, suffix := range []string{"_key", "_secret", "_token", "_password", "_passwd", "_credential", "_credentials"} {
		if strings.HasSuffix(segment, suffix) {
			return true
		}
	}
	return false
}
