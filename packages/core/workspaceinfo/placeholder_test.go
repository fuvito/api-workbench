package workspaceinfo

import (
	"testing"

	"github.com/fuvito/api-workbench/packages/core/domain"
)

func TestWorkspaceSaysWhichSecretHasNoValueYet(t *testing.T) {
	env := &domain.Environment{Name: "e", Variables: []domain.EnvironmentVariable{
		{Name: "token", Value: domain.SecretPlaceholder, Secret: true},
	}}
	r := Inspect(t.TempDir(), env, "e.yaml")
	if len(r.EnvVars) != 1 || r.EnvVars[0].Set {
		t.Errorf("shown as set: %+v", r.EnvVars)
	}
	for _, a := range r.Attention {
		if a.ID == ItemPlaceholderSecret {
			return
		}
	}
	t.Errorf("no item about it: %+v", r.Attention)
}
