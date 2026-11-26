// The CLI Command Builder.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import {
  GetCliCatalog as WailsGetCliCatalog,
  GetCliContext as WailsGetCliContext,
  BuildCliCommand as WailsBuildCliCommand,
  PickCliFile as WailsPickCliFile,
  PickCliFolder as WailsPickCliFolder,
} from '@wailsjs/go/main/CliBuilderService'
import type { CliExplanation } from './core'

// ---- CLI Command Builder ----

export type CliInputKind = 'file' | 'fileOrURL' | 'dir' | 'path' | 'outputFile' | 'url' | 'enum' | 'int' | 'bool' | 'string'

export interface CliInput {
  name: string
  short: string
  positional: boolean
  required: boolean
  kind: CliInputKind
  role: '' | 'scenario' | 'environment' | 'spec' | 'workspace'
  accepts: string[]
  enum: string[]
  default: string
  label: string
  help: string
  example: string
  group: 'essential' | 'advanced'
}

export interface CliExitCode {
  code: number
  meaning: string
  forCI: string
}

export interface CliCommand {
  name: string
  goal: string
  summary: string
  when: string
  inputs: CliInput[]
  outputs: string[]
  exitCodes: CliExitCode[]
}

export interface CliFile {
  path: string
  label: string
}

export interface CliContext {
  workspace: string
  runFrom: string
  scenarios: CliFile[]
  scenarioFolders: CliFile[]
  environments: CliFile[]
  specs: CliFile[]
}

export interface CliBuildInput {
  command: string
  values: Record<string, string>
  runFrom: string
}

export interface CliMessage {
  field: string
  severity: 'error' | 'warning' | 'info'
  text: string
}

export interface CliNeed {
  name: string
  secret: boolean
  from: string
}

export interface CliSuggestion {
  field: string
  value: string
  why: string
}

export type CliShell = 'bash' | 'powershell' | 'cmd'

export interface CliRendering {
  shell: CliShell
  multiline: string
  oneline: string
}

export interface CliBuildOutput {
  command: string
  valid: boolean
  argv: string[]
  renderings: CliRendering[]
  messages: CliMessage[]
  needs: CliNeed[]
  suggestions: CliSuggestion[]
  explanation: CliExplanation
}

export async function getCliCatalog(): Promise<CliCommand[]> {
  return (await WailsGetCliCatalog()) as unknown as CliCommand[]
}

export async function getCliContext(): Promise<CliContext> {
  return (await WailsGetCliContext()) as unknown as CliContext
}

export async function buildCliCommand(input: CliBuildInput): Promise<CliBuildOutput> {
  return (await WailsBuildCliCommand(input as never)) as unknown as CliBuildOutput
}

// `current` is what the field holds now and `runFrom` the folder relative paths are read from: the
// dialog opens in the workspace for an empty field, otherwise where the field points.
export async function pickCliFile(title: string, current = '', runFrom = ''): Promise<string> {
  return WailsPickCliFile(title, current, runFrom)
}

export async function pickCliFolder(title: string, current = '', runFrom = ''): Promise<string> {
  return WailsPickCliFolder(title, current, runFrom)
}
