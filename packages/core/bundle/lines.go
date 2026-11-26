package bundle

import "gopkg.in/yaml.v3"

// StepLines finds the line of every step of every scenario in the text of a bundle, by the
// scenario's position and then the step's. It is nil when the text cannot be read as a document.
func StepLines(data []byte) [][]int {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	var out [][]int
	for _, sc := range sequenceUnder(doc.Content[0], "scenarios") {
		var lines []int
		for _, step := range sequenceUnder(sc, "steps") {
			lines = append(lines, step.Line)
		}
		out = append(out, lines)
	}
	return out
}

func sequenceUnder(n *yaml.Node, key string) []*yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key && n.Content[i+1].Kind == yaml.SequenceNode {
			return n.Content[i+1].Content
		}
	}
	return nil
}
