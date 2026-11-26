package conformance

import "testing"

func limits(t *testing.T, values ...string) Options {
	t.Helper()
	parsed, err := ParsePathLimits(values)
	if err != nil {
		t.Fatal(err)
	}
	return Options{MaxResponseTimeMs: 1000, PathLimits: parsed}
}

func TestTimeLimitPrecedence(t *testing.T) {
	o := limits(t, "get /reports/{id}=10", "/reports=20", "/reports/{id}/=25", "tag:slow=30", "tag:reports=40", "tag:off=0", "POST /orders=0")
	for name, c := range map[string]struct {
		method     string
		tags       []string
		spec, sent string
		want       int
	}{
		"the operation wins":                    {"GET", []string{"slow"}, "/reports/{id}", "/reports/{{id}}", 10},
		"another method takes the path":         {"DELETE", []string{"slow"}, "/reports/{id}", "/reports/{{id}}", 20},
		"an operation is not a prefix":          {"GET", nil, "/reports/{id}/rows", "/reports/{{id}}/rows", 25},
		"a path wins over a tag":                {"GET", []string{"slow"}, "/reports", "/reports", 20},
		"a tag wins over the general limit":     {"GET", []string{"reports"}, "/a", "/a", 40},
		"the strictest of two tags":             {"GET", []string{"reports", "slow"}, "/a", "/a", 30},
		"no limit is the least strict":          {"GET", []string{"off", "reports"}, "/a", "/a", 40},
		"a tag that only switches off":          {"GET", []string{"off"}, "/a", "/a", 0},
		"an operation can switch the limit off": {"POST", []string{"slow"}, "/orders", "/orders", 0},
		"nothing matches":                       {"GET", []string{"other"}, "/a", "/a", 1000},
	} {
		if got := o.limitFor(c.method, c.tags, c.spec, c.sent); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
	// parameters written the generated way name the same operation
	g := limits(t, "GET /reports/{{id}}=7")
	if got := g.limitFor("GET", nil, "/reports/{id}", "/reports/{{id}}"); got != 7 {
		t.Errorf("generated spelling: %d", got)
	}
}

func TestTimeLimitRulesThatCannotBeRead(t *testing.T) {
	for _, bad := range []string{"reports=5", "tag:=5", "tag: =5", "FETCH /a=5", "GET a=5", "GET=5", "/a", "/a=x", "/a=-1", "=5"} {
		if _, err := ParsePathLimits([]string{bad}); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	got, err := ParsePathLimits([]string{" tag: user reports =5", "get  /a=6", "/q?x=1=7"})
	if err != nil || got["tag:user reports"] != 5 || got["GET /a"] != 6 || got["/q?x=1"] != 7 {
		t.Errorf("%v %v", got, err)
	}
}
