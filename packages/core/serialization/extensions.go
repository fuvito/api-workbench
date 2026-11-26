package serialization

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"gopkg.in/yaml.v3"
)

// A save rebuilds a file from the model, so the author's x- keys are read off the document beside
// the file types and put back on the document before it is written (B-153). Every file type that
// has an Extensions field takes part. Bundle types have none, so a bundle never carries them.

// maxExtensionNodes bounds what aliases inside x- values can expand to.
const maxExtensionNodes = 100_000

const extensionPrefix = "x-"

var extensionsType = reflect.TypeOf(domain.Extensions(""))

// readExtensions fills the Extensions fields of file, a pointer to a decoded file type.
func readExtensions(data []byte, format string, file any) error {
	var root *yaml.Node
	if strings.EqualFold(format, "json") {
		n, err := jsonNode(data)
		if err != nil {
			return fmt.Errorf("parsing json: %w", err)
		}
		root = n
	} else {
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parsing yaml: %w", err)
		}
		if len(doc.Content) == 0 {
			return nil
		}
		root = doc.Content[0]
	}
	budget := maxExtensionNodes
	return walkFile(root, reflect.ValueOf(file), func(mapping *yaml.Node, ext reflect.Value) error {
		var lines []string
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			key := mapping.Content[i].Value
			if !strings.HasPrefix(key, extensionPrefix) {
				continue
			}
			var b strings.Builder
			if err := writeNodeJSON(&b, mapping.Content[i+1], &budget); err != nil {
				return fmt.Errorf("%s: %w", printable(key), err)
			}
			lines = append(lines, jsonString(key)+": "+b.String())
		}
		ext.SetString(strings.Join(lines, "\n"))
		return nil
	})
}

// marshalFile encodes a file type, with its extensions, as YAML or JSON.
func marshalFile(file any, format string) ([]byte, error) {
	isJSON := false
	switch strings.ToLower(format) {
	case "yaml", "yml":
	case "json":
		isJSON = true
	default:
		return nil, fmt.Errorf("unknown format %q; expected yaml or json", format)
	}
	if !hasExtensions(reflect.ValueOf(file)) {
		if isJSON {
			return json.MarshalIndent(file, "", "  ")
		}
		return yaml.Marshal(file)
	}

	root := &yaml.Node{}
	if isJSON {
		data, err := json.Marshal(file)
		if err != nil {
			return nil, err
		}
		if root, err = jsonNode(data); err != nil {
			return nil, err
		}
	} else if err := root.Encode(file); err != nil {
		return nil, err
	}
	if err := walkFile(root, reflect.ValueOf(file), appendExtensions); err != nil {
		return nil, err
	}
	if !isJSON {
		return yaml.Marshal(root)
	}
	var compact strings.Builder
	budget := maxExtensionNodes * 10
	if err := writeNodeJSON(&compact, root, &budget); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(compact.String()), "", "  "); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// appendExtensions adds an item's lines to the end of its mapping. Only x- keys are accepted, so
// that text from an editor cannot set a key that something reads.
func appendExtensions(mapping *yaml.Node, ext reflect.Value) error {
	for _, line := range strings.Split(ext.String(), "\n") {
		if line == "" {
			continue
		}
		pair, err := jsonNode([]byte("{" + line + "}"))
		if err != nil || pair.Kind != yaml.MappingNode || len(pair.Content) != 2 ||
			!strings.HasPrefix(pair.Content[0].Value, extensionPrefix) {
			return errors.New("an x- key could not be written back, so nothing was saved")
		}
		mapping.Style = 0 // an empty mapping is written as {}, on one line
		mapping.Content = append(mapping.Content, pair.Content...)
	}
	return nil
}

// walkFile follows the document and the file type together and calls visit for every mapping that
// stands for a struct with an Extensions field.
func walkFile(node *yaml.Node, v reflect.Value, visit func(*yaml.Node, reflect.Value) error) error {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Slice:
		if node.Kind != yaml.SequenceNode {
			return nil
		}
		for i := 0; i < v.Len() && i < len(node.Content); i++ {
			if err := walkFile(node.Content[i], v.Index(i), visit); err != nil {
				return err
			}
		}
	case reflect.Struct:
		if node.Kind != yaml.MappingNode {
			return nil
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
			if name == "" || name == "-" {
				continue
			}
			for j := 0; j+1 < len(node.Content); j += 2 {
				if node.Content[j].Value == name {
					if err := walkFile(node.Content[j+1], v.Field(i), visit); err != nil {
						return err
					}
					break
				}
			}
		}
		// after the fields, so that what visit adds is not walked
		if f := v.FieldByName("Extensions"); f.IsValid() && f.Type() == extensionsType {
			return visit(node, f)
		}
	}
	return nil
}

func hasExtensions(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer:
		return !v.IsNil() && hasExtensions(v.Elem())
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if hasExtensions(v.Index(i)) {
				return true
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).Type() == extensionsType {
				if v.Field(i).String() != "" {
					return true
				}
			} else if hasExtensions(v.Field(i)) {
				return true
			}
		}
	}
	return false
}

func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return strings.TrimSuffix(b.String(), "\n")
}

// writeNodeJSON writes a document value as compact JSON, keeping the order of keys and the digits
// of numbers. What JSON has no type for, such as a date, is written as text.
func writeNodeJSON(b *strings.Builder, n *yaml.Node, budget *int) error {
	if *budget--; *budget < 0 {
		return errors.New("the value expands to too many items")
	}
	switch n.Kind {
	case yaml.AliasNode:
		return writeNodeJSON(b, n.Alias, budget)
	case yaml.MappingNode:
		b.WriteByte('{')
		for i := 0; i+1 < len(n.Content); i += 2 {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(jsonString(n.Content[i].Value))
			b.WriteByte(':')
			if err := writeNodeJSON(b, n.Content[i+1], budget); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case yaml.SequenceNode:
		b.WriteByte('[')
		for i, item := range n.Content {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeNodeJSON(b, item, budget); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	default:
		switch n.ShortTag() {
		case "!!null":
			b.WriteString("null")
			return nil
		case "!!bool", "!!int", "!!float":
			if json.Valid([]byte(n.Value)) {
				b.WriteString(n.Value)
				return nil
			}
			var v any // True, 0x1F, 1_000 and the like
			if err := n.Decode(&v); err == nil {
				if data, err := json.Marshal(v); err == nil {
					b.Write(data)
					return nil
				}
			}
		}
		b.WriteString(jsonString(n.Value))
	}
	return nil
}

// jsonNode reads JSON as a document tree, keeping the order of keys. The YAML parser is not used:
// it refuses some JSON, such as escaped surrogate pairs.
func jsonNode(data []byte) (*yaml.Node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := nextJSONNode(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more than one JSON value")
	}
	return n, nil
}

func nextJSONNode(dec *json.Decoder) (*yaml.Node, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	scalar := func(tag, value string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value} }
	switch t := token.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		if t == '{' {
			n = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		for dec.More() {
			child, err := nextJSONNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		_, err := dec.Token() // the closing delimiter
		return n, err
	case string:
		return scalar("!!str", t), nil
	case json.Number:
		if strings.ContainsAny(t.String(), ".eE") {
			return scalar("!!float", t.String()), nil
		}
		return scalar("!!int", t.String()), nil
	case bool:
		return scalar("!!bool", fmt.Sprint(t)), nil
	default:
		return scalar("!!null", "null"), nil
	}
}
