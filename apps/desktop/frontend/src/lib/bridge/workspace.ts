// The workspace: its files, environments, saved responses, export and import, and the Workspace page.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import {
  OpenWorkspace as WailsOpenWorkspace,
  NewWorkspace as WailsNewWorkspace,
  CloseWorkspace as WailsCloseWorkspace,
  GetWorkspacePath as WailsGetWorkspacePath,
  ListRequests as WailsListRequests,
  LoadRequest as WailsLoadRequest,
  SaveRequest as WailsSaveRequest,
  OpenRequest as WailsOpenRequest,
  ListEnvironments as WailsListEnvironments,
  SelectEnvironment as WailsSelectEnvironment,
  GetActiveEnvironment as WailsGetActiveEnvironment,
  GetCloseReminder as WailsGetCloseReminder,
  GetEnvironment as WailsGetEnvironment,
  ParseEnvironmentYAML as WailsParseEnvironmentYAML,
  SaveEnvironment as WailsSaveEnvironment,
  DeleteEnvironment as WailsDeleteEnvironment,
  GetEnvironmentRaw as WailsGetEnvironmentRaw,
  SaveEnvironmentRaw as WailsSaveEnvironmentRaw,
  ValidateEnvironmentYAML as WailsValidateEnvironmentYAML,
  DeleteRequest as WailsDeleteRequest,
  CreateFolder as WailsCreateFolder,
  DeleteFolder as WailsDeleteFolder,
  RenameFolder as WailsRenameFolder,
  RenameRequest as WailsRenameRequest,
  SaveResponseCache as WailsSaveResponseCache,
  LoadResponseCache as WailsLoadResponseCache,
  ClearResponseCache as WailsClearResponseCache,
  OpenInSystemViewer as WailsOpenInSystemViewer,
  OpenInExplorer as WailsOpenInExplorer,
  MoveRequest as WailsMoveRequest,
  CopyRequest as WailsCopyRequest,
  GetRecentWorkspaces as WailsGetRecentWorkspaces,
  OpenWorkspaceByPath as WailsOpenWorkspaceByPath,
  SaveContentToFile as WailsSaveContentToFile,
  PickFile as WailsPickFile,
  ExportWorkspace as WailsExportWorkspace,
  WorkspaceHasSecrets as WailsWorkspaceHasSecrets,
  ScanWorkspaceImport as WailsScanWorkspaceImport,
  ApplyWorkspaceImport as WailsApplyWorkspaceImport,
} from '@wailsjs/go/main/WorkspaceService'
import { GetWorkspaceOverview as WailsGetWorkspaceOverview } from '@wailsjs/go/main/DashboardService'
import {
  MoveScenario as WailsMoveScenario,
  CopyScenario as WailsCopyScenario,
} from '@wailsjs/go/main/ScenarioService'
import type { DashEnvVar, DashLastRun, EnvInfo, FileNode, RequestFileIO, ResponseData } from './core'

// ---- Workspace ----

export async function openWorkspace(): Promise<string> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsOpenWorkspace()
  return result as unknown as string
}

export async function newWorkspace(): Promise<string> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsNewWorkspace()
  return result as unknown as string
}

export async function closeWorkspace(): Promise<void> {
  await WailsCloseWorkspace()
}

export async function getRecentWorkspaces(): Promise<string[]> {
  const result = await WailsGetRecentWorkspaces()
  return result as unknown as string[]
}

export async function openWorkspaceByPath(path: string): Promise<string> {
  const result = await WailsOpenWorkspaceByPath(path)
  return result as unknown as string
}

export async function saveContentToFile(content: string, defaultName: string): Promise<string> {
  const result = await WailsSaveContentToFile(content, defaultName)
  return result as unknown as string
}

export async function pickFile(title = 'Select file'): Promise<string> {
  const result = await WailsPickFile(title)
  return result as unknown as string
}

export async function getWorkspacePath(): Promise<string> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsGetWorkspacePath()
  return result as unknown as string
}

export async function listRequests(): Promise<FileNode[]> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsListRequests()
  return (result ?? []) as unknown as FileNode[]
}

export async function loadRequest(path: string): Promise<RequestFileIO> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsLoadRequest(path)
  return result as unknown as RequestFileIO
}

export async function saveRequest(req: RequestFileIO): Promise<string> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsSaveRequest(req as any)
  return result as unknown as string
}

export async function openRequest(): Promise<RequestFileIO | null> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsOpenRequest()
  return result as unknown as RequestFileIO | null
}

// ---- Environments ----

export async function listEnvironments(): Promise<EnvInfo[]> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsListEnvironments()
  return (result ?? []) as unknown as EnvInfo[]
}

export async function selectEnvironment(path: string): Promise<void> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  await WailsSelectEnvironment(path)
}

export async function getActiveEnvironment(): Promise<EnvInfo | null> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsGetActiveEnvironment()
  return result as unknown as EnvInfo | null
}

export interface EnvironmentVariable {
  name: string
  value: string
  secret: boolean
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export interface EnvironmentIO {
  path: string
  name: string
  variables: EnvironmentVariable[]
  /** The item's x- keys from its file, carried with it and written back on save; never read here. */
  extensions?: string
}

export async function parseEnvironmentYAML(content: string): Promise<EnvironmentIO> {
  const result = await WailsParseEnvironmentYAML(content)
  return result as unknown as EnvironmentIO
}

export async function getEnvironment(path: string): Promise<EnvironmentIO> {
  const result = await WailsGetEnvironment(path)
  return result as unknown as EnvironmentIO
}

export async function saveEnvironment(env: EnvironmentIO): Promise<EnvInfo> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsSaveEnvironment(env as any)
  return result as unknown as EnvInfo
}

export async function deleteEnvironment(path: string): Promise<void> {
  await WailsDeleteEnvironment(path)
}

export async function getEnvironmentRaw(path: string): Promise<string> {
  const result = await WailsGetEnvironmentRaw(path)
  return result as unknown as string
}

export async function saveEnvironmentRaw(path: string, content: string): Promise<EnvInfo> {
  const result = await WailsSaveEnvironmentRaw(path, content)
  return result as unknown as EnvInfo
}

export async function validateEnvironmentYAML(content: string): Promise<string> {
  return (await WailsValidateEnvironmentYAML(content)) as unknown as string
}

export async function deleteRequest(path: string): Promise<void> {
  await WailsDeleteRequest(path)
}

export async function createFolder(parentPath: string, name: string): Promise<string> {
  const result = await WailsCreateFolder(parentPath, name)
  return result as unknown as string
}

export async function deleteFolder(path: string): Promise<void> {
  await WailsDeleteFolder(path)
}

export async function renameFolder(path: string, newName: string): Promise<string> {
  const result = await WailsRenameFolder(path, newName)
  return result as unknown as string
}

export async function renameRequest(path: string, newName: string): Promise<string> {
  const result = await WailsRenameRequest(path, newName)
  return result as unknown as string
}

// ---- Response cache ----

export interface ResponseCacheData extends ResponseData {
  cachedAt: string
}

export async function saveResponseCache(reqPath: string, response: ResponseData): Promise<void> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  await WailsSaveResponseCache(reqPath, response as any)
}

export async function openInSystemViewer(content: string, format: string): Promise<string> {
  const result = await WailsOpenInSystemViewer(content, format)
  return result as unknown as string
}

export async function openInExplorer(path: string): Promise<string> {
  const result = await WailsOpenInExplorer(path)
  return result as unknown as string
}

export async function moveRequest(srcPath: string, destDir: string): Promise<string> {
  const result = await WailsMoveRequest(srcPath, destDir)
  return result as unknown as string
}

export async function copyRequest(srcPath: string, destDir: string): Promise<string> {
  const result = await WailsCopyRequest(srcPath, destDir)
  return result as unknown as string
}

export async function moveScenario(srcPath: string, destFolderRelPath: string): Promise<string> {
  const result = await WailsMoveScenario(srcPath, destFolderRelPath)
  return result as unknown as string
}

export async function copyScenario(srcPath: string, destFolderRelPath: string): Promise<string> {
  const result = await WailsCopyScenario(srcPath, destFolderRelPath)
  return result as unknown as string
}

export async function loadResponseCache(reqPath: string): Promise<ResponseCacheData | null> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result = await WailsLoadResponseCache(reqPath)
  return result as unknown as ResponseCacheData | null
}

export async function clearResponseCache(reqPath: string): Promise<void> {
  await WailsClearResponseCache(reqPath)
}

// ---- Workspace export / import ----

export interface ScanImportResult {
  zipPath: string       // empty = user cancelled
  totalFiles: number
  conflicts: string[]   // relative paths that already exist
  error?: string
}

export interface ApplyImportResult {
  imported: number
  skipped: number
  renamed: number
  error?: string
}

// Returns "" on success or an error message. Empty string also returned on cancel.
export async function exportWorkspace(): Promise<string> {
  return (await WailsExportWorkspace()) as unknown as string
}

export async function workspaceHasSecrets(): Promise<boolean> {
  return (await WailsWorkspaceHasSecrets()) as unknown as boolean
}

export async function scanWorkspaceImport(): Promise<ScanImportResult> {
  const result = await WailsScanWorkspaceImport()
  return result as unknown as ScanImportResult
}

export async function applyWorkspaceImport(zipPath: string, strategy: string): Promise<ApplyImportResult> {
  const result = await WailsApplyWorkspaceImport(zipPath, strategy)
  return result as unknown as ApplyImportResult
}

// ---- Workspace page ----

export type AttentionItem = {
  id: string
  severity: 'warning' | 'info'
  what: string
  where: string
  why: string
  fix: string
  target: { kind: 'scenario' | 'environment' | 'spec' | 'request' | 'contracts' | 'projectfile'; path: string }
}

export type DashCoverage = {
  ref: string
  total: number
  covered: number
  notCovered: number
  passing: number
  failing: number
  lastRunAt?: string
}

export type DashSpec = {
  ref: string
  path?: string
  state: 'found' | 'missing' | 'outside' | 'url'
  scenarios: string[]
  // every spelling scenarios used for this spec
  refs: string[]
  // one entry per stored Contracts card for it (usually one; empty when nothing was recorded)
  coverage: DashCoverage[]
}

export type EnvironmentSource = 'remembered' | 'chosen' | 'temporary' | 'none'

export type WorkspaceOverview = {
  open: boolean
  path: string
  name: string
  temporary: boolean
  requests: number
  scenarios: number
  environments: number
  environmentName?: string
  environmentPath?: string
  environmentSource: EnvironmentSource
  envVars: DashEnvVar[]
  specs: DashSpec[]
  lastRun?: DashLastRun
  attention: AttentionItem[]
  // the clrsig.yaml in the workspace folder
  projectFile?: { found: boolean; path?: string; error?: string; keys: string[] }
  snapshotAt: string
}

export async function getWorkspaceOverview(): Promise<WorkspaceOverview> {
  return (await WailsGetWorkspaceOverview()) as unknown as WorkspaceOverview
}

export async function getCloseReminder(): Promise<string> {
  return WailsGetCloseReminder()
}
