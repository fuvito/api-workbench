package domain

const CurrentSchemaVersion = "1"

// Environment holds named variables that are substituted into requests at runtime.
// Switching environments changes variable values without modifying request definitions.
type Environment struct {
	SchemaVersion string
	Name          string
	Variables     []EnvironmentVariable
}

type EnvironmentVariable struct {
	Name   string
	Value  string
	Secret bool // redacted in UI, logs, and reports; value should reference an env var in CI/CD
}

func NewEnvironment(name string) *Environment {
	return &Environment{
		SchemaVersion: CurrentSchemaVersion,
		Name:          name,
	}
}

// SecretPlaceholder is what bundle export writes in place of a secret value.
const SecretPlaceholder = "CHANGE_ME"

// IsPlaceholder reports a secret that still holds SecretPlaceholder. It stands for a value that was
// left out, so everything that fills in values treats the variable as not there: sending the text
// as the secret would be wrong.
func (v EnvironmentVariable) IsPlaceholder() bool {
	return v.Secret && v.Value == SecretPlaceholder
}

// Lookup returns the value for the given variable name and whether it was found.
func (e *Environment) Lookup(name string) (string, bool) {
	for _, v := range e.Variables {
		if v.Name == name {
			return v.Value, true
		}
	}
	return "", false
}
