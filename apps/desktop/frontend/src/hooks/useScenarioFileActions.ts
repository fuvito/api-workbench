import { useState, type ComponentProps } from 'react'
import type { Sidebar } from '@/components/Sidebar'
import {
  createScenario, createScenarioFolder, deleteScenarioFolder, duplicateScenario, renameScenario, renameScenarioFolder,
} from '@/lib/bridge'
import type { Tabs, Workspace } from '@/hooks/appTypes'

type Params = {
  tabs: Tabs
  workspace: Workspace
  /** Asks first; the app deletes once it is confirmed. */
  onAskDelete: (scenario: { path: string; name: string; isDirty: boolean }) => void
}

/** What the Scenarios sidebar does with scenario files and folders. */
export function useScenarioFileActions({ tabs, workspace, onAskDelete: setDeletingScenario }: Params) {
  const [scenarioRenameError, setScenarioRenameError] = useState<string | null>(null)

  const scenarioActions: Pick<ComponentProps<typeof Sidebar>,
    'onNewScenario' | 'onNewScenarioFolder' | 'onRenameScenarioFolder' | 'onDeleteScenarioFolder' | 'onNewScenarioFile' | 'onDuplicateScenario' | 'onDeleteScenario' | 'onRenameScenario'
  > = {
    onNewScenario: async (name, folderPath) => {
      const result = await createScenario(name, folderPath ?? '')
      if (!result.error && result.path) {
        await workspace.refreshWorkspace()
        tabs.openScenarioAsTab(result.path, name)
      }
    },
    onNewScenarioFolder: async (parentFolderPath, name) => {
      const relPath = parentFolderPath ? `${parentFolderPath}/${name}` : name
      await createScenarioFolder(relPath)
      await workspace.refreshWorkspace()
    },
    onRenameScenarioFolder: async (folderPath, newName) => {
      const result = await renameScenarioFolder(folderPath, newName)
      if (!result.startsWith('error:')) {
        await workspace.refreshWorkspace()
      }
    },
    onDeleteScenarioFolder: async (folderPath) => {
      const scenDir = `${workspace.workspacePath.replace(/\\/g, '/')}/scenarios/`
      const matchingTabs = tabs.tabs.filter(t =>
        t.kind === 'scenario' && (t.scenarioPath ?? '').replace(/\\/g, '/').startsWith(scenDir + folderPath)
      )
      const err = await deleteScenarioFolder(folderPath)
      if (!err) {
        for (const t of matchingTabs) tabs.removeTab(t.id)
        await workspace.refreshWorkspace()
      }
    },
    onNewScenarioFile: async (path, name) => {
      await workspace.refreshWorkspace()
      tabs.openScenarioAsTab(path, name)
    },
    onDuplicateScenario: async (path) => {
      const result = await duplicateScenario(path)
      if (!result.error && result.path) {
        await workspace.refreshWorkspace()
        const name = result.path.split(/[\\/]/).pop()?.replace(/\.(yaml|yml|json)$/i, '') ?? 'copy'
        tabs.openScenarioAsTab(result.path, name)
      }
    },
    onDeleteScenario: (path) => {
      const matchTab = tabs.tabs.find(t => t.kind === 'scenario' && t.scenarioPath === path)
      const name = workspace.scenarioList.find(s => s.path === path)?.title ?? path.split(/[\\/]/).pop()?.replace(/\.(yaml|yml|json)$/i, '') ?? 'scenario'
      setDeletingScenario({ path, name, isDirty: matchTab?.scenarioDirty ?? false })
    },
    onRenameScenario: async (path, newName) => {
      const result = await renameScenario(path, newName)
      if (!result.startsWith('error:')) {
        const newPath = result
        tabs.setTabsState(prev => ({
          ...prev,
          tabs: prev.tabs.map(t =>
            t.kind === 'scenario' && t.scenarioPath === path
              ? { ...t, scenarioPath: newPath, scenarioName: newName, name: newName }
              : t
          ),
        }))
        await workspace.refreshWorkspace()
      } else {
        setScenarioRenameError(result.replace(/^error:\s*/i, ''))
        setTimeout(() => setScenarioRenameError(null), 4000)
      }
    },
  }

  return { scenarioRenameError, scenarioActions }
}
