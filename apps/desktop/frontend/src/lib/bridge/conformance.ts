// Check Conformance and the local servers.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import {
  StartDemo as WailsStartDemo,
  StopDemo as WailsStopDemo,
  IsRunning as WailsIsDemoRunning,
  GetPort as WailsGetDemoPort,
  OpenSpecInBrowser as WailsOpenDemoSpec,
} from '@wailsjs/go/main/DemoService'
import { EventsOn } from '@wailsjs/runtime/runtime'
import {
  GetConformContext as WailsGetConformContext,
  InspectConformSpec as WailsInspectConformSpec,
  RunConformance as WailsRunConformance,
  CancelConformance as WailsCancelConformance,
  ExportConformReport as WailsExportConformReport,
  RecordConformBaseline as WailsRecordConformBaseline,
} from '@wailsjs/go/main/ConformanceService'
import {
  Stop as WailsStopConformanceServer,
  OpenWorkspace as WailsOpenConformanceWorkspace,
  OpenSpecInBrowser as WailsOpenConformanceSpec,
} from '@wailsjs/go/main/ConformanceServerService'
import type { CliFile } from './cli'
import type { RunReport } from './scenarios'

// ---- Demo server ----

export interface DemoResult {
  workspacePath: string
  port: number
  error?: string
}

export async function startDemo(): Promise<DemoResult> {
  const result = await WailsStartDemo()
  return result as unknown as DemoResult
}

export async function stopDemo(): Promise<void> {
  await WailsStopDemo()
}

export async function isDemoRunning(): Promise<boolean> {
  const result = await WailsIsDemoRunning()
  return result as unknown as boolean
}

export async function getDemoPort(): Promise<number> {
  const result = await WailsGetDemoPort()
  return result as unknown as number
}

export async function openDemoSpec(): Promise<string> {
  const result = await WailsOpenDemoSpec()
  return result as unknown as string
}

// ---- Check conformance ----

export interface ConformContext {
  workspace: string
  specs: CliFile[]
  environments: CliFile[]
  // the file "Active environment" means; empty when none is selected
  activeEnvironment?: string
  // what the workspace's clrsig.yaml chooses for this page
  projectDefaults?: ConformDefaults
  // workspace scenarios that can run first (a login, for example)
  scenarios?: CliFile[]
  // workspace files that look like conformance baselines
  baselines?: CliFile[]
}

export interface ConformDefaults {
  found: boolean
  file?: string
  // what was not applied and why
  note?: string
  // the keys that gave a value, as written in the file
  applied: string[]
  specPath?: string
  environmentPath?: string
  baseUrl?: string
  setupPath?: string
  baselinePath?: string
  tags: string[]
  paths: string[]
  negative: boolean
  failOnWarning: boolean
  requestTimeoutMs: number
  maxResponseTimeMs: number
  maxResponseTimeFor: string[]
}

export interface ConformSpecInfo {
  title: string
  version: string
  serverUrl: string
  reads: number
  writes: number
  error?: string
  // problems in the spec document itself
  lint?: SpecIssue[]
}

export interface SpecIssue {
  check: string
  severity: 'error' | 'warning'
  where: string
  what: string
  why: string
  fix: string
}

export interface ConformRunInput {
  specPath: string
  baseUrl: string
  envPath: string
  allowWrites: boolean
  confirmedRemote: boolean
  tags: string[]
  paths: string[]
  negative: boolean
  failFast: boolean
  // a warning fails the run (clrsig conform --fail-on warning)
  failOnWarning?: boolean
  // a workspace scenario that runs first; what it extracts is available to the requests as {{name}}
  setupScenario?: string
  // a baseline file in the workspace: recorded violations are accepted and only new ones fail the run
  baselinePath?: string
  maxResponseTimeMs?: number
  maxResponseTimeFor?: string[]
  timeoutMs: number
  recordCoverage: boolean
  vars: string[]
}

export interface ConformProbe {
  operationId: string
  method: string
  path: string
  kind: string
  description: string
  status: 'rejected' | 'accepted' | 'error' | 'skipped' | 'not-checked'
  statusCode: number
  error?: string
  /** What is doubtful: refused for what may be another reason, or not checked (429). */
  warning?: string
}

export interface ConformFinding {
  kind: 'missing-field' | 'wrong-type' | 'undeclared-status' | 'wrong-content-type' | 'missing-header' | 'bad-header' | 'missing-value' | 'dynamic-value-failed' | 'no-branch-matched' | 'invalid-body' | 'empty-body' | 'body-too-large' | 'constraint' | 'unexpected-property' | 'write-only-property' | 'number-out-of-range' | 'request' | 'no-response' | 'other'
  // the check's ID in the catalog (Help, What We Check), for example CS-RESP-005
  checkId?: string
  severity?: string
  // where the problem is: the part (response body, response status, response header, request,
  // network) and what in it (a field path, a header name, the status code)
  part?: string
  where?: string
  // where the expectation comes from in the spec, for example "GET /a → response 200 → headers → X-Total-Count"
  spec?: string
  summary: string
  expected?: string
  got?: string
  why: string
  fix: string
  detail?: string
}

export interface ConformOperation {
  operationId: string
  method: string
  path: string
  // the path as the spec writes it, which the page shows (absent in older data)
  specPath?: string
  // blocked: not sent because a {{placeholder}} had no value
  status: 'passed' | 'failed' | 'error' | 'blocked' | 'skipped'
  statusCode: number
  durationMs: number
  violations: string[]
  error?: string
  // plain-language explanation of each violation (absent in older data)
  findings?: ConformFinding[]
  // findings that are only warnings; a passing operation can have some
  warnings?: number
}

export interface ConformRunOutput {
  error?: string
  specTitle: string
  baseUrl: string
  writes: boolean
  durationMs: number
  operations: ConformOperation[]
  total: number
  covered: number
  notCovered: number
  passing: number
  failing: number
  // passing operations that have warnings
  withWarnings?: number
  probes: ConformProbe[]
  warnings: string[]
  // placeholders that kept operations from being sent (each once) and how many operations that hit
  missingValues?: string[]
  blockedOperations?: number
  coverageNote?: string
  // the run's verdict, as the CLI's exit code 1
  failed?: boolean
  // what the setup scenario made available (names, never values)
  setupNote?: string
  // how the run compares with the chosen baseline
  baseline?: ConformBaseline
}

export interface ConformViolation {
  operation: string
  check?: string
  where?: string
  summary: string
}

export interface ConformBaseline {
  path: string
  // a version 1 file: it still works; recording a new baseline upgrades it
  legacy: boolean
  known: ConformViolation[]
  new: ConformViolation[]
  fixed: ConformViolation[]
}

export interface RecordedBaseline {
  path: string
  violations: number
  replaced: boolean
  error?: string
}

export type ConformReportFormat = 'html' | 'junit' | 'json' | 'markdown'

export async function getConformContext(): Promise<ConformContext> {
  return (await WailsGetConformContext()) as unknown as ConformContext
}

export async function inspectConformSpec(specPath: string): Promise<ConformSpecInfo> {
  return (await WailsInspectConformSpec(specPath)) as unknown as ConformSpecInfo
}

export async function runConformance(input: ConformRunInput): Promise<ConformRunOutput> {
  return (await WailsRunConformance(input as never)) as unknown as ConformRunOutput
}

export async function cancelConformance(): Promise<void> {
  await WailsCancelConformance()
}

export async function recordConformBaseline(path: string): Promise<RecordedBaseline> {
  return (await WailsRecordConformBaseline(path)) as unknown as RecordedBaseline
}

export async function exportConformReport(format: ConformReportFormat): Promise<RunReport> {
  return (await WailsExportConformReport(format)) as unknown as RunReport
}

export interface ConformProgress {
  done: number
  total: number
  operation: string
}

// Calls cb after each operation of a conformance run ("conformance:progress"). Returns the
// function that stops listening. Outside the desktop app (tests, the browser mock) it does nothing.
export function onConformanceProgress(cb: (progress: ConformProgress) => void): () => void {
  try {
    const off = EventsOn('conformance:progress', cb) as unknown
    return typeof off === 'function' ? (off as () => void) : () => {}
  } catch {
    return () => {}
  }
}

// ---- Conformance server ----

export interface ConformanceServerResult {
  port: number
  workspacePath?: string
  error?: string
}

export async function openConformanceWorkspace(): Promise<ConformanceServerResult> {
  const result = await WailsOpenConformanceWorkspace()
  return result as unknown as ConformanceServerResult
}

export async function stopConformanceServer(): Promise<void> {
  await WailsStopConformanceServer()
}

export async function openConformanceSpec(): Promise<string> {
  const result = await WailsOpenConformanceSpec()
  return result as unknown as string
}
