package main

import (
	"context"
	"sync"

	"github.com/fuvito/api-workbench/packages/core/scenario"
	"github.com/fuvito/api-workbench/packages/core/variables"
)

// ---- Service ----

type ScenarioService struct {
	ctx       context.Context
	workspace *WorkspaceService
	settings  *SettingsService
	mu        sync.Mutex
	cancelRun context.CancelFunc
	// cancelFolder stops the request-folder run in progress (RunFolder), if any.
	cancelFolder context.CancelFunc
	// runs keeps the latest result of each scenario that was run, so a run report can be
	// exported from the Go reporting package after a Run All or a folder run.
	runs map[string]*scenario.Result
}

func NewScenarioService(ws *WorkspaceService, ss *SettingsService) *ScenarioService {
	return &ScenarioService{ctx: context.Background(), workspace: ws, settings: ss}
}

func (s *ScenarioService) setContext(ctx context.Context) {
	s.ctx = ctx
}

// DynamicFunctionIO describes one {{$function}} for the editors' autocomplete.
type DynamicFunctionIO struct {
	Name        string `json:"name"`
	Usage       string `json:"usage"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// ListDynamicFunctions returns the built-in dynamic-value functions, straight from the core so the
// editors never drift from what a run accepts.
func (s *ScenarioService) ListDynamicFunctions() []DynamicFunctionIO {
	infos := variables.Functions()
	out := make([]DynamicFunctionIO, len(infos))
	for i, f := range infos {
		out[i] = DynamicFunctionIO{Name: f.Name, Usage: f.Usage, Description: f.Description, Example: f.Example}
	}
	return out
}
