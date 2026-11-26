// Shared types, and the calls for the app itself, sending a request, history and settings.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import {
  SetHasDirtyTabs as WailsSetHasDirtyTabs,
  Exit as WailsExit,
} from '@wailsjs/go/main/App'
import {
  Execute as WailsExecute,
  GetHistory as WailsGetHistory,
  CancelRequest as WailsCancelRequest,
  ClearHistory as WailsClearHistory,
} from '@wailsjs/go/main/RequestService'
import {
  GetSettings as WailsGetSettings,
  SaveSettings as WailsSaveSettings,
  ResetSettings as WailsResetSettings,
} from '@wailsjs/go/main/SettingsService'
import type { CliExitCode } from './cli'

// ---- Shared I/O types ----

/** A request header as Go sends and receives it. */
export interface Header {
  name: string
  value: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface QueryParam {
  name: string
  value: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

/** A header or query parameter of a scenario step's request. */
export interface KVPair {
  name: string
  value: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface RequestBody {
  type: string
  content: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface CliPart {
  token: string
  meaning: string
}

export interface DashEnvVar {
  name: string
  secret: boolean
  set: boolean
  unsetRef?: string
}

export interface DashLastRun {
  at: string
  scenario: string
  passed: boolean
  scenarios: number
  failingNow: number
}

export interface OperationResult {
  lastPassed: boolean
  lastRunAt: string
  lastDurationMs?: number
  slowestDurationMs?: number
}

export interface CliExplanation {
  goal: string
  summary: string
  when: string
  parts: CliPart[]
  outputs: string[]
  exitCodes: CliExitCode[]
}

export interface AuthPayload {
  type: string
  token: string
  username: string
  password: string
  key: string
  value: string
  location: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface RequestPayload {
  method: string
  url: string
  headers: Header[]
  queryParams: QueryParam[]
  body?: RequestBody
  auth?: AuthPayload
  timeoutMs: number
  noFollowRedirects?: boolean
}

export interface TimingData {
  dnsMs: number
  connectMs: number
  tlsMs: number
  ttfbMs: number
  transferMs: number
}

export interface ResponseData {
  statusCode: number
  status: string
  headers: Header[]
  body: string
  durationMs: number
  size: number
  timing: TimingData
  error?: string
  // the request as it was sent, credentials masked by the Go core; not stored in the response cache
  requestMethod?: string
  requestUrl?: string
  requestHeaders?: Header[]
  requestBody?: string
}

export interface AppSettings {
  theme: 'dark' | 'light'
  fontSize: number
  defaultTimeoutMs: number
  proxyUrl: string
  tlsSkipVerify: boolean
  telemetryEnabled: boolean
  // mutual TLS: file paths only (may contain ${NAME}); never the certificate contents
  clientCertFile?: string
  clientKeyFile?: string
  caFile?: string
  // Dev Server ports; 0 or absent = pick a free port automatically
  demoServerPort?: number
  conformanceServerPort?: number
}

export interface HistoryEntry {
  id: number
  executedAt: string
  method: string
  url: string
  statusCode: number
  statusText: string
  durationMs: number
  sizeBytes: number
  headers: Header[]
  bodyType: string
  bodyContent: string
  auth?: AuthPayload
}

export interface FileNode {
  name: string
  path: string
  isDir: boolean
  method?: string
  children?: FileNode[]
}

export interface EnvInfo {
  name: string
  path: string
}

export interface RequestFileIO {
  schemaVersion: string
  title: string
  method: string
  url: string
  headers?: Header[]
  queryParams?: QueryParam[]
  body?: RequestBody
  auth?: AuthPayload
  timeoutMs?: number
  noFollowRedirects?: boolean
  filePath?: string
  targetDir?: string
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

// ---- App lifecycle ----

export async function setHasDirtyTabs(dirty: boolean): Promise<void> {
  await WailsSetHasDirtyTabs(dirty)
}

export async function quitApp(): Promise<void> {
  await WailsExit()
}

// ---- Request execution ----

export async function executeRequest(payload: RequestPayload): Promise<ResponseData> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsExecute(payload as any)
  return result as unknown as ResponseData
}

// ---- History ----

export async function getHistory(): Promise<HistoryEntry[]> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsGetHistory()
  return (result ?? []) as unknown as HistoryEntry[]
}

export async function cancelRequest(): Promise<void> {
  await WailsCancelRequest()
}

export async function clearHistory(): Promise<void> {
  await WailsClearHistory()
}

// ---- Settings ----

export async function getSettings(): Promise<AppSettings> {
  const result = await WailsGetSettings()
  return result as unknown as AppSettings
}

export async function saveSettings(s: AppSettings): Promise<AppSettings> {
  const result = await WailsSaveSettings(s as unknown as any) // eslint-disable-line @typescript-eslint/no-explicit-any
  return result as unknown as AppSettings
}

export async function resetSettings(): Promise<AppSettings> {
  const result = await WailsResetSettings()
  return result as unknown as AppSettings
}
