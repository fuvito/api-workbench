package serialization

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// A key nothing reads is refused (B-121): a misspelled "assertions" used to leave a step that
// checked nothing and passed. Keys that start with x- are ignored, for the author's own use. The
// known keys are the yaml tags of the file types, so a new field is accepted as soon as it exists.

// legacyKeys are keys releases 0.1.0 to 0.3.0 had and nothing reads any more. Their files must
// keep working.
var legacyKeys = map[reflect.Type][]string{
	reflect.TypeOf(StepFile{}):    {"title", "description"},
	reflect.TypeOf(RequestFile{}): {"description"},
}

// likelyMeant answers mistakes a spelling distance does not find.
var likelyMeant = map[reflect.Type]map[string]string{
	reflect.TypeOf(ScenarioFile{}):   {"name": "title"},
	reflect.TypeOf(StepFile{}):       {"name": "label", "extract": "extractions", "assert": "assertions", "asserts": "assertions"},
	reflect.TypeOf(ExtractionFile{}): {"from": "source"},
	reflect.TypeOf(RequestFile{}):    {"name": "title", "timeoutms": "timeout", "params": "queryParams", "query": "queryParams"},
}

const maxReported = 5

// checkKeys refuses keys that file type does not have. A file that cannot be decoded is left to
// the decoder that reports why.
func checkKeys(data []byte, format string, file any) error {
	var doc any
	var err error
	if strings.EqualFold(format, "json") {
		err = json.Unmarshal(data, &doc)
	} else {
		err = yaml.Unmarshal(data, &doc)
	}
	if err != nil {
		return nil
	}
	var found []string
	walkKeys(doc, reflect.TypeOf(file), "", &found)
	if len(found) == 0 {
		return nil
	}
	if extra := len(found) - maxReported; extra > 0 {
		found = append(found[:maxReported], fmt.Sprintf("and %d more", extra))
	}
	return errors.New(strings.Join(found, "; ") + ". Keys that start with x- are ignored, for your own use")
}

func walkKeys(value any, t reflect.Type, where string, found *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice:
		if list, ok := value.([]any); ok {
			for i, item := range list {
				walkKeys(item, t.Elem(), fmt.Sprintf("%s[%d]", where, i), found)
			}
		}
	case reflect.Struct:
		object, ok := asObject(value)
		if !ok {
			return
		}
		fields := map[string]reflect.Type{}
		var names []string
		for i := 0; i < t.NumField(); i++ {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
			if name != "" && name != "-" {
				fields[name] = t.Field(i).Type
				names = append(names, name)
			}
		}
		keys := make([]string, 0, len(object))
		for k := range object {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if field, known := fields[k]; known {
				inner := k
				if where != "" {
					inner = where + "." + k
				}
				walkKeys(object[k], field, inner, found)
				continue
			}
			if strings.HasPrefix(k, "x-") || contains(legacyKeys[t], k) {
				continue
			}
			message := fmt.Sprintf("unknown key %q", printable(k))
			if where != "" {
				message += " in " + where
			}
			if meant := suggest(k, names, likelyMeant[t]); meant != "" {
				message += fmt.Sprintf(" (did you mean %q?)", meant)
			}
			*found = append(*found, message)
		}
	}
}

func asObject(value any) (map[string]any, bool) {
	switch m := value.(type) {
	case map[string]any:
		return m, true
	case map[any]any: // YAML with a key that is not text
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[fmt.Sprint(k)] = v
		}
		return out, true
	}
	return nil, false
}

// printable cuts a key that is really a line missing its space, such as "password:hunter2", so
// that an error never repeats a value.
func printable(key string) string {
	if i := strings.IndexAny(key, ":= \t\n"); i >= 0 {
		key = key[:i] + "…"
	}
	if r := []rune(key); len(r) > 40 {
		key = string(r[:40]) + "…"
	}
	return key
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func suggest(key string, known []string, hints map[string]string) string {
	lower := strings.ToLower(key)
	if meant := hints[lower]; meant != "" {
		return meant
	}
	best, bestD := "", 3
	for _, k := range known {
		if d := EditDistance(lower, strings.ToLower(k)); d < bestD && d < len(k) && d < len(key) {
			best, bestD = k, d
		}
	}
	return best
}

// EditDistance is the number of single-letter changes between a and b.
func EditDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	row := make([]int, len(y)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(x); i++ {
		diagonal := row[0]
		row[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			next := min(row[j]+1, row[j-1]+1, diagonal+cost)
			diagonal, row[j] = row[j], next
		}
	}
	return row[len(y)]
}
