package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/serialization"
)

// The bundled scenario templates.

//go:embed scenario_templates/*.yaml
var scenarioTemplateFS embed.FS

// ---- Scenario template gallery ----

// TemplateInfoIO describes a bundled scenario template available for the user to copy.
type TemplateInfoIO struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	StepCount   int      `json:"stepCount"`
	// Note is shown as a small callout on the card (e.g. "Requires {{baseUrl}}").
	Note string `json:"note,omitempty"`
}

// categoryAndTags derives category and tag metadata from the template filename.
func templateMeta(id string) (category string, tags []string, note string) {
	conformanceIDs := map[string]bool{
		"user-lifecycle":          true,
		"pagination":              true,
		"401-www-authenticate":    true,
		"405-method-not-allowed":  true,
		"503-service-unavailable": true,
		"content-negotiation":     true,
		"etag-conditional":        true,
		"problem-details":         true,
		"rate-limit":              true,
	}
	if conformanceIDs[id] {
		return "Conformance", []string{"RFC", "headers"}, "Requires {{baseUrl}} (use with demo server)"
	}
	switch id {
	case "jsonplaceholder-crud":
		return "Public APIs", []string{"CRUD", "REST", "no auth"}, ""
	case "github-public-api":
		return "Public APIs", []string{"GitHub", "no auth"}, ""
	}
	return "Other", nil, ""
}

// ListTemplates returns all bundled scenario templates.
func (s *ScenarioService) ListTemplates() []TemplateInfoIO {
	entries, err := scenarioTemplateFS.ReadDir("scenario_templates")
	if err != nil {
		return nil
	}
	var out []TemplateInfoIO
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yaml")
		data, err := scenarioTemplateFS.ReadFile("scenario_templates/" + e.Name())
		if err != nil {
			continue
		}
		sc, err := serialization.ParseScenarioFile(data, e.Name())
		if err != nil {
			continue
		}
		desc := strings.TrimSpace(sc.Description)
		// Truncate long descriptions for the card preview.
		if len(desc) > 160 {
			desc = desc[:157] + "…"
		}
		category, tags, note := templateMeta(id)
		out = append(out, TemplateInfoIO{
			ID:          id,
			Title:       sc.Title,
			Description: desc,
			Category:    category,
			Tags:        tags,
			StepCount:   len(sc.Steps),
			Note:        note,
		})
	}
	return out
}

// CreateFromTemplate copies a bundled template into the workspace scenarios directory
// under the given name and returns the new file path.
func (s *ScenarioService) CreateFromTemplate(templateID, name string) CreateScenarioResultIO {
	if templateID == "" {
		return CreateScenarioResultIO{Error: "templateID is required"}
	}
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return CreateScenarioResultIO{Error: "no workspace open"}
	}

	srcPath := "scenario_templates/" + templateID + ".yaml"
	data, err := scenarioTemplateFS.ReadFile(srcPath)
	if err != nil {
		return CreateScenarioResultIO{Error: "template not found: " + templateID}
	}

	// Parse and re-serialise with the new title so the YAML reflects the user's chosen name.
	sc, err := serialization.ParseScenarioFile(data, templateID+".yaml")
	if err != nil {
		return CreateScenarioResultIO{Error: "parsing template: " + err.Error()}
	}
	if strings.TrimSpace(name) != "" {
		sc.Title = strings.TrimSpace(name)
	}

	scenDir := filepath.Join(wsPath, "scenarios")
	if err := os.MkdirAll(scenDir, 0755); err != nil {
		return CreateScenarioResultIO{Error: err.Error()}
	}

	slug := toSlug(sc.Title)
	if slug == "" {
		slug = templateID
	}
	filePath := filepath.Join(scenDir, slug+".yaml")
	for i := 2; ; i++ {
		if _, err := os.Stat(filePath); err != nil {
			break
		}
		filePath = filepath.Join(scenDir, fmt.Sprintf("%s-%d.yaml", slug, i))
	}

	out, err := serialization.WriteScenario(sc, "yaml")
	if err != nil {
		return CreateScenarioResultIO{Error: "serialising template: " + err.Error()}
	}
	if err := atomicWrite(filePath, out); err != nil {
		return CreateScenarioResultIO{Error: err.Error()}
	}
	return CreateScenarioResultIO{Path: filePath}
}
