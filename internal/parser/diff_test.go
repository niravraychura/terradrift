package parser

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/terraform"
)

func TestAttributeChangesIncludeOldAndNewValues(t *testing.T) {
	plan := []byte(`{
		"resource_changes":[{
			"address":"aws_lb.main",
			"type":"aws_lb",
			"name":"main",
			"mode":"managed",
			"change":{
				"actions":["update"],
				"before":{"idle_timeout":60,"tags":{"Environment":"staging"},"name":"main"},
				"after":{"idle_timeout":120,"tags":{"Environment":"dev"},"name":"main"},
				"before_sensitive":false,
				"after_sensitive":false
			}
		}]
	}`)
	changes, _, _, _, err := ParsePlan(plan, terraform.PlanModeNormal)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %#v", changes)
	}
	attrs := map[string]report.AttributeChange{}
	for _, attr := range changes[0].AttributeChanges {
		attrs[attr.Path] = attr
	}
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attribute diffs, got %#v", changes[0].AttributeChanges)
	}
	if got := attrs["idle_timeout"]; got.Before != "60" || got.After != "120" {
		t.Fatalf("idle_timeout = %#v", got)
	}
	if got := attrs["tags.Environment"]; got.Before != `"staging"` || got.After != `"dev"` {
		t.Fatalf("tags.Environment = %#v", got)
	}
}

func TestAttributeChangesRedactUnknownAndCreateDelete(t *testing.T) {
	plan := []byte(`{
		"resource_changes":[
			{
				"address":"aws_instance.created",
				"type":"aws_instance",
				"name":"created",
				"mode":"managed",
				"change":{
					"actions":["create"],
					"before":null,
					"after":{"ami":"ami-123","token":"abc"},
					"after_unknown":{"id":true}
				}
			},
			{
				"address":"aws_instance.deleted",
				"type":"aws_instance",
				"name":"deleted",
				"mode":"managed",
				"change":{
					"actions":["delete"],
					"before":{"ami":"ami-old"},
					"after":null
				}
			}
		]
	}`)
	changes, _, _, _, err := ParsePlan(plan, terraform.PlanModeNormal)
	if err != nil || len(changes) != 2 {
		t.Fatalf("parse plan: changes=%#v err=%v", changes, err)
	}
	created := map[string]report.AttributeChange{}
	for _, attr := range changes[0].AttributeChanges {
		created[attr.Path] = attr
	}
	if got := created["ami"]; got.Before != "(absent)" || got.After != `"ami-123"` {
		t.Fatalf("create ami = %#v", got)
	}
	if got := created["token"]; got.Before != "(absent)" || got.After != "[REDACTED]" {
		t.Fatalf("create token = %#v", got)
	}
	if got := created["id"]; got.Before != "(absent)" || got.After != "(known after apply)" {
		t.Fatalf("create id = %#v", got)
	}
	deleted := map[string]report.AttributeChange{}
	for _, attr := range changes[1].AttributeChanges {
		deleted[attr.Path] = attr
	}
	if got := deleted["ami"]; got.Before != `"ami-old"` || got.After != "(absent)" {
		t.Fatalf("delete ami = %#v", got)
	}
}

func TestAttributeChangesRedactConnectionString(t *testing.T) {
	plan := []byte(`{
		"resource_changes":[{
			"address":"aws_db_instance.main",
			"type":"aws_db_instance",
			"name":"main",
			"mode":"managed",
			"change":{
				"actions":["update"],
				"before":{"connection_string":"postgres://user:secret@db/prod","engine":"postgres"},
				"after":{"connection_string":"postgres://user:other@db/prod","engine":"postgres"}
			}
		}]
	}`)
	changes, _, _, _, err := ParsePlan(plan, terraform.PlanModeNormal)
	if err != nil || len(changes) != 1 {
		t.Fatalf("parse plan: %#v err=%v", changes, err)
	}
	attrs := map[string]report.AttributeChange{}
	for _, attr := range changes[0].AttributeChanges {
		attrs[attr.Path] = attr
	}
	if got := attrs["connection_string"]; got.Before != "[REDACTED]" || got.After != "[REDACTED]" {
		t.Fatalf("connection_string = %#v", got)
	}
	encoded := string(mustMarshal(t, changes))
	if strings.Contains(encoded, "postgres://") || strings.Contains(encoded, "secret") {
		t.Fatalf("connection string leaked: %s", encoded)
	}
}

func TestAttributeChangesSummarizeLargeBlobs(t *testing.T) {
	blob := strings.Repeat("a", 4128)
	plan := []byte(`{
		"resource_changes":[{
			"address":"aws_iam_policy.main",
			"type":"aws_iam_policy",
			"name":"main",
			"mode":"managed",
			"change":{
				"actions":["update"],
				"before":{"policy":` + mustQuoteJSON(blob) + `,"name":"main"},
				"after":{"policy":` + mustQuoteJSON(blob+"x") + `,"name":"main"}
			}
		}]
	}`)
	changes, _, _, _, err := ParsePlan(plan, terraform.PlanModeNormal)
	if err != nil || len(changes) != 1 {
		t.Fatalf("parse plan: %#v err=%v", changes, err)
	}
	attrs := map[string]report.AttributeChange{}
	for _, attr := range changes[0].AttributeChanges {
		attrs[attr.Path] = attr
	}
	if got := attrs["policy"]; got.Before != "[changed, 4128B]" || got.After != "[changed, 4129B]" {
		t.Fatalf("policy summary = %#v", got)
	}
	if strings.Contains(gotString(attrs["policy"]), "aaaa") {
		t.Fatal("expected blob summary without partial dump")
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func mustQuoteJSON(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func gotString(attr report.AttributeChange) string {
	return attr.Before + attr.After
}
