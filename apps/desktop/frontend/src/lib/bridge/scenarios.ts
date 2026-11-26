// Scenarios: files, runs, reports and templates.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import type { DynamicFunction } from '@/lib/dynamicFunctions'
import {
  ListScenarios as WailsListScenarios,
  LoadScenario as WailsLoadScenario,
  RunScenario as WailsRunScenario,
  RunFolder as WailsRunFolder,
  CancelFolderRun as WailsCancelFolderRun,
  LoadScenarioRaw as WailsLoadScenarioRaw,
  SaveScenarioRaw as WailsSaveScenarioRaw,
  ValidateScenarioYAML as WailsValidateScenarioYAML,
  CreateScenario as WailsCreateScenario,
  DuplicateScenario as WailsDuplicateScenario,
  DeleteScenario as WailsDeleteScenario,
  RenameScenario as WailsRenameScenario,
  CreateScenarioFolder as WailsCreateScenarioFolder,
  RenameScenarioFolder as WailsRenameScenarioFolder,
  DeleteScenarioFolder as WailsDeleteScenarioFolder,
  SaveScenarioResult as WailsSaveScenarioResult,
  LoadScenarioResult as WailsLoadScenarioResult,
  ListTemplates as WailsListTemplates,
  CreateFromTemplate as WailsCreateFromTemplate,
  GetAccumulatedCoverage as WailsGetAccumulatedCoverage,
  ResetAccumulatedCoverage as WailsResetAccumulatedCoverage,
  ListAccumulatedCoverage as WailsListAccumulatedCoverage,
  ExportCoverageReport as WailsExportCoverageReport,
  ExportRunReport as WailsExportRunReport,
  ListDynamicFunctions as WailsListDynamicFunctions,
  GetScenarioRunHistory as WailsGetScenarioRunHistory,
  CancelScenario as WailsCancelScenario,
} from '@wailsjs/go/main/ScenarioService'
import type { ConformFinding } from './conformance'
import type { AuthPayload, Header, KVPair, OperationResult } from './core'

// ---- Scenarios ----

export interface ScenarioInfo {
  title: string
  path: string
  folder?: string  // relative path from {workspace}/scenarios/, "/" separated, "" or absent for root
  stepCount?: number
  isFolder?: boolean  // sentinel: represents a directory entry, not a scenario file
}

export interface AssertionDetail {
  type: string
  path?: string
  operator?: string
  expected?: unknown
  schema?: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
  name?: string
}

export interface ExtractionDetail {
  name: string
  source: string
  path?: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface StepDetail {
  label: string
  notes?: string
  disabled?: boolean
  method?: string
  url?: string
  headers?: KVPair[]
  queryParams?: KVPair[]
  bodyType?: string
  bodyContent?: string
  requestRef?: string
  assertions: AssertionDetail[]
  extractions: ExtractionDetail[]
  failPolicy?: string
  timeoutMs?: number
  retryMaxAttempts?: number
  retryDelayMs?: number
  noFollowRedirects?: boolean
  auth?: AuthPayload
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
  /** What the editor does not show of the step's request, body and retry, kept for saving. */
  requestTitle?: string
  requestTimeout?: string
  requestExtensions?: string
  bodyExtensions?: string
  retryExtensions?: string
}

export interface ScenarioDetail {
  path: string
  title: string
  description: string
  specFile?: string
  variables?: ScenarioVariable[]
  steps: StepDetail[]
  error?: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface ScenarioVariable {
  name: string
  value: string
  secret?: boolean
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface AssertionResult {
  type: string
  passed: boolean
  message: string
}

export interface StepResult {
  label: string
  passed: boolean
  skipped: boolean
  statusCode: number
  durationMs: number
  error?: string
  extracted: Record<string, string>
  assertions: AssertionResult[]
  responseBody?: string
  responseHeaders?: Header[]
  attempts?: number
  // the request as it was sent (variables and dynamic values filled in, credentials masked)
  requestMethod?: string
  requestUrl?: string
  requestHeaders?: Header[]
  requestBody?: string
  // spec conformance (absent when the scenario has no spec)
  specState?: 'matched' | 'violations' | 'not-checked'
  specReason?: string
  specViolations?: string[]
  specFindings?: ConformFinding[]
}

export interface ScenarioCoverage {
  covered: number
  total: number
  percent: number
  exercised: string[]
  missed: string[]
}

export interface QualityGap {
  operation: string
  missing: number[]
}

export interface QualityReport {
  neverExercised: string[]
  happyPathOnly: string[]
  securedNeverAuthed: string[]
  errorsNeverTriggered: QualityGap[]
}

export interface AccumulatedCoverage {
  specFile: string
  covered: number
  total: number
  percent: number
  totalRuns: number
  passedRuns: number
  failedRuns: number
  lastRunAt?: string
  exercised: string[]
  missed: string[]
  operationResults?: Record<string, OperationResult>
  quality?: QualityReport
}

export interface ScenarioResult {
  title: string
  passed: boolean
  durationMs: number
  steps: StepResult[]
  error?: string
  specError?: string
  coverage?: ScenarioCoverage
  accumulatedCoverage?: AccumulatedCoverage
  seed?: number
  variables?: ScenarioVariable[]
}

export async function listDynamicFunctions(): Promise<DynamicFunction[]> {
  return ((await WailsListDynamicFunctions()) ?? []) as unknown as DynamicFunction[]
}

export async function listScenarios(): Promise<ScenarioInfo[]> {
  const result = await WailsListScenarios()
  return (result ?? []) as unknown as ScenarioInfo[]
}

export async function loadScenario(path: string): Promise<ScenarioDetail> {
  const result = await WailsLoadScenario(path)
  return result as unknown as ScenarioDetail
}

export async function runScenario(path: string, envPath = '', fromStep = 0): Promise<ScenarioResult> {
  const result = await WailsRunScenario(path, envPath, fromStep)
  return result as unknown as ScenarioResult
}

export async function cancelScenario(): Promise<void> {
  await WailsCancelScenario()
}

export async function loadScenarioRaw(path: string): Promise<string> {
  return (await WailsLoadScenarioRaw(path)) as unknown as string
}

export async function saveScenarioRaw(path: string, content: string): Promise<string> {
  return (await WailsSaveScenarioRaw(path, content)) as unknown as string
}

export async function validateScenarioYAML(content: string): Promise<string> {
  return (await WailsValidateScenarioYAML(content)) as unknown as string
}

export interface CreateScenarioResult {
  path: string
  error?: string
}

export async function createScenario(name: string, folderPath = ''): Promise<CreateScenarioResult> {
  const result = await WailsCreateScenario(name, folderPath)
  return result as unknown as CreateScenarioResult
}

// Returns "" on success or an error message.
export async function createScenarioFolder(folderPath: string): Promise<string> {
  return (await WailsCreateScenarioFolder(folderPath)) as unknown as string
}

// Returns the new relative folder path on success, or "error: ..." on failure.
export async function renameScenarioFolder(folderPath: string, newName: string): Promise<string> {
  return (await WailsRenameScenarioFolder(folderPath, newName)) as unknown as string
}

// Returns "" on success or an error message.
export async function deleteScenarioFolder(folderPath: string): Promise<string> {
  return (await WailsDeleteScenarioFolder(folderPath)) as unknown as string
}

// Returns "" on success or an error message.
export async function saveScenarioResult(scenarioPath: string, result: ScenarioResult): Promise<void> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  await WailsSaveScenarioResult(scenarioPath, result as any)
}

// Returns null if no cached result exists.
export async function loadScenarioResult(scenarioPath: string): Promise<ScenarioResult | null> {
  const r = await WailsLoadScenarioResult(scenarioPath)
  return r as unknown as ScenarioResult | null
}

export async function duplicateScenario(path: string): Promise<CreateScenarioResult> {
  const result = await WailsDuplicateScenario(path)
  return result as unknown as CreateScenarioResult
}

// Returns "" on success, or an error message.
export async function deleteScenario(path: string): Promise<string> {
  return (await WailsDeleteScenario(path)) as unknown as string
}

// Returns the new file path on success, or a string starting with "error:" on failure.
export async function renameScenario(path: string, newName: string): Promise<string> {
  return (await WailsRenameScenario(path, newName)) as unknown as string
}

// ---- Scenario templates ----

export interface TemplateInfo {
  id: string
  title: string
  description: string
  category: string
  tags: string[]
  stepCount: number
  note?: string
}

export async function listTemplates(): Promise<TemplateInfo[]> {
  const result = await WailsListTemplates()
  return (result ?? []) as unknown as TemplateInfo[]
}

export async function createFromTemplate(templateID: string, name: string): Promise<CreateScenarioResult> {
  const result = await WailsCreateFromTemplate(templateID, name)
  return result as unknown as CreateScenarioResult
}

export type RunReportFormat = 'html' | 'junit' | 'json' | 'markdown' | 'text'

export interface RunReport {
  content: string
  error?: string
}

// Renders the latest results of the given scenarios with the Go reporting package, the same code
// (and the same redaction of secrets) the CLI uses.
export async function exportRunReport(format: RunReportFormat, paths: string[]): Promise<RunReport> {
  return (await WailsExportRunReport(format, paths)) as unknown as RunReport
}

export type CoverageReportFormat = 'csv' | 'markdown' | 'html'

export interface CoverageReport {
  content: string
  error?: string
}

// The report is rendered by the Go core, with the same code the CLI's coverage command uses.
export async function exportCoverageReport(specFile: string, format: CoverageReportFormat): Promise<CoverageReport> {
  return (await WailsExportCoverageReport(specFile, format)) as unknown as CoverageReport
}

export async function getAccumulatedCoverage(specFile: string): Promise<AccumulatedCoverage | null> {
  const result = await WailsGetAccumulatedCoverage(specFile)
  return result as unknown as AccumulatedCoverage | null
}

export async function resetAccumulatedCoverage(specFile: string): Promise<void> {
  await WailsResetAccumulatedCoverage(specFile)
}

export interface StepRunEntry {
  label: string
  passed: boolean
  skipped: boolean
  statusCode?: number
  durationMs?: number
  error?: string
}

export interface ScenarioRunEntry {
  id: number
  executedAt: string
  scenarioPath: string
  title: string
  passed: boolean
  durationMs: number
  stepTotal: number
  stepPassed: number
  stepFailed: number
  stepSkipped: number
  steps: StepRunEntry[]
}

export async function getScenarioRunHistory(limit = 100): Promise<ScenarioRunEntry[]> {
  const result = await WailsGetScenarioRunHistory(limit)
  return (result ?? []) as unknown as ScenarioRunEntry[]
}

export async function listAccumulatedCoverage(): Promise<AccumulatedCoverage[]> {
  const result = await WailsListAccumulatedCoverage()
  return (result ?? []) as unknown as AccumulatedCoverage[]
}

// ---- Folder run ----

export interface FolderRunItem {
  name: string
  path: string
  method: string
  statusCode: number
  durationMs: number
  error?: string
  passed: boolean
}

export async function cancelFolderRun(): Promise<void> {
  await WailsCancelFolderRun()
}

export async function runFolder(folderPath: string): Promise<FolderRunItem[]> {
  const result = await WailsRunFolder(folderPath)
  return (result ?? []) as unknown as FolderRunItem[]
}
