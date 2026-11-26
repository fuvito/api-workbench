package main

// What the Scenarios screens send and receive.

// ---- I/O types ----

type ScenarioInfoIO struct {
	Title     string `json:"title"`
	Path      string `json:"path"`
	Folder    string `json:"folder"` // relative path from {workspace}/scenarios/, "/" separated, "" for root
	StepCount int    `json:"stepCount"`
	IsFolder  bool   `json:"isFolder,omitempty"` // sentinel: represents an empty directory, not a file
}

type KVPairIO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

type AssertionDetailIO struct {
	Type     string      `json:"type"`
	Path     string      `json:"path,omitempty"`
	Operator string      `json:"operator,omitempty"`
	Expected interface{} `json:"expected,omitempty"`
	Schema   string      `json:"schema,omitempty"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
	Name       string `json:"name,omitempty"`
}

type ExtractionDetailIO struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

type StepDetailIO struct {
	Label            string               `json:"label"`
	Notes            string               `json:"notes,omitempty"`
	Disabled         bool                 `json:"disabled,omitempty"`
	Method           string               `json:"method,omitempty"`
	URL              string               `json:"url,omitempty"`
	Headers          []KVPairIO           `json:"headers"`
	QueryParams      []KVPairIO           `json:"queryParams"`
	BodyType         string               `json:"bodyType,omitempty"`
	BodyContent      string               `json:"bodyContent,omitempty"`
	RequestRef       string               `json:"requestRef,omitempty"`
	Assertions       []AssertionDetailIO  `json:"assertions"`
	Extractions      []ExtractionDetailIO `json:"extractions"`
	FailPolicy       string               `json:"failPolicy,omitempty"`
	TimeoutMs        int                  `json:"timeoutMs,omitempty"`
	RetryMaxAttempts int                  `json:"retryMaxAttempts,omitempty"`
	RetryDelayMs     int                  `json:"retryDelayMs,omitempty"`
	// NoFollowRedirects is the step request's followRedirects: false.
	NoFollowRedirects bool `json:"noFollowRedirects,omitempty"`
	// Auth is the step request's auth block; the editor keeps it so saving does not drop it.
	Auth *AuthIO `json:"auth,omitempty"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
	// what the editor does not show of the step's request, body and retry, kept for saving
	RequestTitle      string `json:"requestTitle,omitempty"`
	RequestTimeout    string `json:"requestTimeout,omitempty"`
	RequestExtensions string `json:"requestExtensions,omitempty"`
	BodyExtensions    string `json:"bodyExtensions,omitempty"`
	RetryExtensions   string `json:"retryExtensions,omitempty"`
}

// VariableIO is a scenario variable; the editor keeps them so saving does not drop them.
type VariableIO struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret,omitempty"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

type ScenarioDetailIO struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	SpecFile    string         `json:"specFile,omitempty"`
	Variables   []VariableIO   `json:"variables,omitempty"`
	Steps       []StepDetailIO `json:"steps"`
	Error       string         `json:"error,omitempty"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

type AssertionResultIO struct {
	Type    string `json:"type"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type StepResultIO struct {
	Label           string              `json:"label"`
	Passed          bool                `json:"passed"`
	Skipped         bool                `json:"skipped"`
	StatusCode      int                 `json:"statusCode"`
	DurationMs      int64               `json:"durationMs"`
	Error           string              `json:"error,omitempty"`
	Extracted       map[string]string   `json:"extracted"`
	Assertions      []AssertionResultIO `json:"assertions"`
	ResponseBody    string              `json:"responseBody,omitempty"`
	ResponseHeaders []HeaderIO          `json:"responseHeaders,omitempty"`
	// Attempts is set only when the step was re-sent by its retry policy.
	Attempts int `json:"attempts,omitempty"`
	// The request as it was sent, after variables and dynamic values were filled in. Credentials
	// are masked (see exchangeRequest).
	RequestMethod  string     `json:"requestMethod,omitempty"`
	RequestURL     string     `json:"requestUrl,omitempty"`
	RequestHeaders []HeaderIO `json:"requestHeaders,omitempty"`
	RequestBody    string     `json:"requestBody,omitempty"`
	// Spec conformance: empty when the scenario has no spec; otherwise matched, violations or
	// not-checked (with the reason).
	SpecState      string   `json:"specState,omitempty"`
	SpecReason     string   `json:"specReason,omitempty"`
	SpecViolations []string `json:"specViolations,omitempty"`
	// SpecFindings explain each violation the way the Check Conformance page does (what the spec
	// expected, what came back, where, why and how to fix it); SpecViolations is the validator's raw text.
	SpecFindings []ConformFindingIO `json:"specFindings,omitempty"`
}

type CoverageIO struct {
	Covered   int      `json:"covered"`
	Total     int      `json:"total"`
	Percent   float64  `json:"percent"`
	Exercised []string `json:"exercised"`
	Missed    []string `json:"missed"`
}

type ScenarioResultIO struct {
	Title               string                 `json:"title"`
	Passed              bool                   `json:"passed"`
	DurationMs          int64                  `json:"durationMs"`
	Steps               []StepResultIO         `json:"steps"`
	Error               string                 `json:"error,omitempty"`
	SpecError           string                 `json:"specError,omitempty"`
	Coverage            *CoverageIO            `json:"coverage,omitempty"`
	AccumulatedCoverage *AccumulatedCoverageIO `json:"accumulatedCoverage,omitempty"`
	// Seed is set when the run used random values; run the CLI with --seed to replay them.
	Seed *int64 `json:"seed,omitempty"`
	// Variables are the scenario variables as generated for this run (secret values are masked).
	Variables []VariableIO `json:"variables,omitempty"`
}

type StepRunEntryIO struct {
	Label      string `json:"label"`
	Passed     bool   `json:"passed"`
	Skipped    bool   `json:"skipped"`
	StatusCode int    `json:"statusCode,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ScenarioRunEntryIO struct {
	ID           int64            `json:"id"`
	ExecutedAt   string           `json:"executedAt"`
	ScenarioPath string           `json:"scenarioPath"`
	Title        string           `json:"title"`
	Passed       bool             `json:"passed"`
	DurationMs   int64            `json:"durationMs"`
	StepTotal    int              `json:"stepTotal"`
	StepPassed   int              `json:"stepPassed"`
	StepFailed   int              `json:"stepFailed"`
	StepSkipped  int              `json:"stepSkipped"`
	Steps        []StepRunEntryIO `json:"steps"`
}
