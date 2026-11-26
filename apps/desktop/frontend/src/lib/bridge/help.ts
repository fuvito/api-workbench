// The Help panel.
// Part of lib/bridge.ts, which is what the rest of the app imports.
import {
  GetUserGuide as WailsGetUserGuide,
  GetFormatSpec as WailsGetFormatSpec,
  GetWhatWeCheck as WailsGetWhatWeCheck,
  GetCheckTable as WailsGetCheckTable,
  GetExampleScenario as WailsGetExampleScenario,
  GetAIPrompt as WailsGetAIPrompt,
  GetAIPromptInstructions as WailsGetAIPromptInstructions,
} from '@wailsjs/go/main/HelpService'

// ---- Help ----

export async function getUserGuide(): Promise<string> {
  return (await WailsGetUserGuide()) as unknown as string
}

export interface CheckCell {
  label: string
  /** Markdown */
  text: string
}

export interface CheckRow {
  id: string
  title: string
  section: string
  severity?: string
  cells: CheckCell[]
}

export async function getCheckTable(): Promise<CheckRow[]> {
  return (await WailsGetCheckTable()) as unknown as CheckRow[]
}

export async function getWhatWeCheck(): Promise<string> {
  return (await WailsGetWhatWeCheck()) as unknown as string
}

export async function getFormatSpec(): Promise<string> {
  return (await WailsGetFormatSpec()) as unknown as string
}

export async function getExampleScenario(): Promise<string> {
  return (await WailsGetExampleScenario()) as unknown as string
}

// The AI prompt document as shown in the Help panel (instructions only).
export async function getAIPromptInstructions(): Promise<string> {
  return (await WailsGetAIPromptInstructions()) as unknown as string
}

// The complete prompt to paste into an AI assistant: instructions + format spec + example.
export async function getAIPrompt(): Promise<string> {
  return (await WailsGetAIPrompt()) as unknown as string
}
