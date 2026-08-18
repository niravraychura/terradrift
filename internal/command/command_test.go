package command

import "testing"

func TestValidateRejectsShellTokens(t *testing.T) {
	if Validate("sh -c", nil, nil) == nil {
		t.Fatal("expected shell syntax rejection")
	}
}
