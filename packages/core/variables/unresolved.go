package variables

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/domain"
)

var placeholderRef = regexp.MustCompile(`\{\{\s*([^{}\s][^{}]*?)\s*\}\}`)

// MissingSecret says why a request must not go out when it uses a secret that still holds the
// placeholder of a bundle export; nil when it uses none.
func MissingSecret(env *domain.Environment, req *domain.Request) error {
	held := map[string]bool{}
	for _, name := range Placeholders(env) {
		held[name] = true
	}
	for _, name := range UnresolvedPlaceholders(req) {
		if held[name] {
			return fmt.Errorf("not sent: %s: put the real value in the environment", PlaceholderMessage(name))
		}
	}
	return nil
}

// NotSendable says why a request, already filled in by r, must not go out: a value that could not be
// made, or a {{name}} that would reach the server with its braces. howToFix ends the message with
// where the person can give the value. nil means it can be sent.
func NotSendable(r *Resolver, env *domain.Environment, req *domain.Request, howToFix string) error {
	if errs := r.TakeErrors(); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("not sent: %s", strings.Join(msgs, "; "))
	}
	if err := MissingSecret(env, req); err != nil {
		return err
	}
	if missing := UnresolvedPlaceholders(req); len(missing) > 0 {
		return fmt.Errorf("not sent: no value for {{%s}} (%s)", strings.Join(missing, "}}, {{"), howToFix)
	}
	return nil
}

// UnresolvedPlaceholders lists, in order and without repeats, the {{names}} still present in a
// request after variables were filled in: the ones nothing in the environment or the overrides
// provided. Dynamic functions ({{$name}}) are not listed; their failures are reported separately.
func UnresolvedPlaceholders(req *domain.Request) []string {
	var texts []string
	texts = append(texts, req.URL)
	for _, q := range req.QueryParams {
		texts = append(texts, q.Name, q.Value)
	}
	for _, h := range req.Headers {
		texts = append(texts, h.Name, h.Value)
	}
	if req.Body != nil {
		texts = append(texts, req.Body.Content)
	}
	if a := req.Auth; a != nil {
		if a.Bearer != nil {
			texts = append(texts, a.Bearer.Token)
		}
		if a.Basic != nil {
			texts = append(texts, a.Basic.Username, a.Basic.Password)
		}
		if a.APIKey != nil {
			texts = append(texts, a.APIKey.Key, a.APIKey.Value)
		}
	}
	var names []string
	seen := map[string]bool{}
	for _, text := range texts {
		for _, m := range placeholderRef.FindAllStringSubmatch(text, -1) {
			name := strings.TrimSpace(m[1])
			if strings.HasPrefix(name, "$") || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}
