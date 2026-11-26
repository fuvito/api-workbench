// Importing API descriptions, collections and bundles.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import {
  ImportOpenAPISpec as WailsImportOpenAPISpec,
  ImportOpenAPISpecFromPath as WailsImportOpenAPISpecFromPath,
  ImportOpenAPISpecFromURL as WailsImportOpenAPISpecFromURL,
} from '@wailsjs/go/main/OpenAPIService'
import {
  ImportPostmanCollection as WailsImportPostmanCollection,
  ImportInsomniaExport as WailsImportInsomniaExport,
} from '@wailsjs/go/main/ImportService'
import {
  PickBundleFile as WailsPickBundleFile,
  PreviewBundle as WailsPreviewBundle,
  ImportBundle as WailsImportBundle,
  BundleImportCommand as WailsBundleImportCommand,
  ExportBundle as WailsExportBundle,
} from '@wailsjs/go/main/BundleService'

// ---- OpenAPI import ----

export interface ImportedRequest {
  operationId: string
  summary: string
  method: string
  url: string
  savedPath: string
  tags?: string[]
}

export interface ImportResult {
  specTitle: string
  specVersion: string
  imported: ImportedRequest[]
  error?: string
}

export async function importOpenAPISpec(baseURL = ''): Promise<ImportResult> {
  const result = await WailsImportOpenAPISpec(baseURL)
  return result as unknown as ImportResult
}

export async function importOpenAPISpecFromPath(path: string, baseURL = ''): Promise<ImportResult> {
  const result = await WailsImportOpenAPISpecFromPath(path, baseURL)
  return result as unknown as ImportResult
}

export async function importOpenAPISpecFromURL(url: string, baseURL = ''): Promise<ImportResult> {
  const result = await WailsImportOpenAPISpecFromURL(url, baseURL)
  return result as unknown as ImportResult
}

// ---- Collection import (Postman / Insomnia) ----

export interface CollectionImportResult {
  collectionName: string
  imported: ImportedRequest[]
  skipped: number
  error?: string
  /** What could not be converted, one "request: what" line each. */
  warnings?: string[]
}

export async function importPostmanCollection(): Promise<CollectionImportResult> {
  const result = await WailsImportPostmanCollection()
  return result as unknown as CollectionImportResult
}

export async function importInsomniaExport(): Promise<CollectionImportResult> {
  const result = await WailsImportInsomniaExport()
  return result as unknown as CollectionImportResult
}

// ---- Bundle import (environments + requests + scenarios in one document) ----

export type BundleStrategy = 'skip' | 'overwrite' | 'rename'

export interface BundleFile {
  path: string   // empty = the user cancelled
  text: string
  error?: string
}

export interface BundleIssue {
  severity: 'error' | 'warning'
  where: string
  message: string
}

export interface BundleEntry {
  kind: 'environment' | 'request' | 'scenario'
  name: string
  path: string           // where it will be written, relative to the workspace
  requestedPath: string  // where the bundle asked for it
  action: 'create' | 'overwrite' | 'rename' | 'skip'
}

export interface BundlePreview {
  title: string
  specFile: string       // the bundle's own default spec
  issues: BundleIssue[]
  entries: BundleEntry[]
  hasErrors: boolean
  parseError?: string
}

export interface BundleImportResult {
  created: number
  overwritten: number
  renamed: number
  skipped: number
  written: string[]
  error?: string
}

export interface BundleExportResult {
  path: string            // empty with no error = the user cancelled
  environments: number
  requests: number
  scenarios: number
  redactedSecrets: number
  skipped: string[]       // files that could not be exported, with the reason
  warnings: string[]
  error?: string
}

// Saves the open workspace as one bundle file chosen in a save dialog. Secret values are redacted.
export async function exportBundle(): Promise<BundleExportResult> {
  return (await WailsExportBundle()) as unknown as BundleExportResult
}

export async function pickBundleFile(): Promise<BundleFile> {
  return (await WailsPickBundleFile()) as unknown as BundleFile
}

export async function previewBundle(text: string, specFile: string, strategy: BundleStrategy): Promise<BundlePreview> {
  return (await WailsPreviewBundle(text, specFile, strategy)) as unknown as BundlePreview
}

export interface BundleCommand {
  command: string
  shell: string
  note?: string
  error?: string
}

export async function bundleImportCommand(filePath: string, specFile: string, strategy: BundleStrategy): Promise<BundleCommand> {
  return (await WailsBundleImportCommand(filePath, specFile, strategy)) as unknown as BundleCommand
}

export async function importBundle(text: string, specFile: string, strategy: BundleStrategy): Promise<BundleImportResult> {
  return (await WailsImportBundle(text, specFile, strategy)) as unknown as BundleImportResult
}
