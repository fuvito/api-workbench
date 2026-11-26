import { useState } from 'react'
import { copyRequest, copyScenario, moveRequest, moveScenario } from '@/lib/bridge'
import type { FileClipboard } from '@/types'
import type { Tabs, Workspace } from '@/hooks/appTypes'

type Params = {
  tabs: Tabs
  workspace: Workspace
  onError: (message: string) => void
}

/** Copy, cut, paste and move of request and scenario files. */
export function useFileClipboard({ tabs, workspace, onError: setAppError }: Params) {
  // File clipboard for copy/cut/paste operations
  const [fileClipboard, setFileClipboard] = useState<FileClipboard | null>(null)

  // ── File clipboard ──────────────────────────────────────────────────────────

  async function handlePasteToRequestFolder(destDir: string) {
    if (!fileClipboard || fileClipboard.kind !== 'request') return
    try {
      if (fileClipboard.op === 'copy') {
        await copyRequest(fileClipboard.srcPath, destDir)
      } else {
        await moveRequest(fileClipboard.srcPath, destDir)
        setFileClipboard(null)
      }
      await workspace.refreshWorkspace()
    } catch (e) { setAppError(String(e).replace(/^Error: /, '')) }
  }

  async function handlePasteToScenarioFolder(relFolderPath: string) {
    if (!fileClipboard || fileClipboard.kind !== 'scenario') return
    try {
      if (fileClipboard.op === 'copy') {
        await copyScenario(fileClipboard.srcPath, relFolderPath)
      } else {
        const newPath = await moveScenario(fileClipboard.srcPath, relFolderPath)
        tabs.setTabsState(prev => ({
          ...prev,
          tabs: prev.tabs.map(t =>
            t.kind === 'scenario' && t.scenarioPath === fileClipboard.srcPath
              ? { ...t, scenarioPath: newPath }
              : t
          ),
        }))
        setFileClipboard(null)
      }
      await workspace.refreshWorkspace()
    } catch (e) { setAppError(String(e).replace(/^Error: /, '')) }
  }

  async function handleMoveScenario(srcPath: string, destFolderRelPath: string) {
    try {
      const newPath = await moveScenario(srcPath, destFolderRelPath)
      if (newPath) {
        tabs.setTabsState(prev => ({
          ...prev,
          tabs: prev.tabs.map(t =>
            t.kind === 'scenario' && t.scenarioPath === srcPath
              ? { ...t, scenarioPath: newPath }
              : t
          ),
        }))
        await workspace.refreshWorkspace()
      }
    } catch (e) { setAppError(String(e).replace(/^Error: /, '')) }
  }

  return { fileClipboard, setFileClipboard, handlePasteToRequestFolder, handlePasteToScenarioFolder, handleMoveScenario }
}
