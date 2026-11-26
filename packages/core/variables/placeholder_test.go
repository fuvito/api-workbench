package variables

import (
	"strings"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/domain"
)

func TestASecretHoldingTheExportPlaceholderHasNoValue(t *testing.T) {
	env := &domain.Environment{Variables: []domain.EnvironmentVariable{
		{Name: "token", Value: domain.SecretPlaceholder, Secret: true},
		{Name: "word", Value: domain.SecretPlaceholder}, // not marked secret: an ordinary value
		{Name: "key", Value: "real", Secret: true},
	}}
	r := NewResolver(env)
	if got := r.Interpolate("{{token}} {{word}} {{key}}"); got != "{{token}} CHANGE_ME real" {
		t.Errorf("filled in as %q", got)
	}
	resolved, errs := ResolveEnvironment(env, NewEngine(1))
	if len(errs) != 0 || len(resolved.Variables) != 2 {
		t.Errorf("resolved once for a command: %v %+v", errs, resolved.Variables)
	}
	if got := SecretValues(env); len(got) != 1 || got[0] != "real" {
		t.Errorf("the placeholder is not a secret to mask: %v", got)
	}
	if got := Placeholders(env); len(got) != 1 || got[0] != "token" {
		t.Errorf("named: %v", got)
	}

	uses := &domain.Request{URL: "http://x", Headers: []domain.Header{{Name: "A", Value: "{{token}}"}}}
	if err := MissingSecret(env, r.InterpolateRequest(uses)); err == nil || !strings.Contains(err.Error(), `"token" still holds`) {
		t.Errorf("a request that uses it: %v", err)
	}
	// another undefined name is not this check's business
	other := &domain.Request{URL: "http://x/{{nowhere}}"}
	if err := MissingSecret(env, r.InterpolateRequest(other)); err != nil {
		t.Errorf("a request that does not: %v", err)
	}

	given, err := WithOverrides(env, []string{"token=given"})
	if err != nil || len(Placeholders(given)) != 0 || NewResolver(given).Interpolate("{{token}}") != "given" {
		t.Errorf("--var gives it a value: %v %v", err, Placeholders(given))
	}
}
