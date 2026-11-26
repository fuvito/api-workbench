import { useState } from 'react'
import { createScenario, loadScenario, saveScenarioRaw, type ScenarioDetail } from '@/lib/bridge'
import { serializeScenarioToYAML } from '@/components/scenario-yaml'
import type { Tabs, Workspace } from '@/hooks/appTypes'

type Params = {
  tabs: Tabs
  workspace: Workspace
  showScenarios: () => void
  onError: (message: string) => void
}

/** Makes a scenario step of the response on screen, in a new scenario or an existing one. */
export function useAddToScenario({ tabs, workspace, showScenarios, onError: setAppError }: Params) {
  // New scenario from response dialog
  const [pendingAddToScenario, setPendingAddToScenario] = useState<{ defaultName: string } | null>(null)
  const [newScenarioName, setNewScenarioName] = useState('')

  function openAddToScenarioDialog() {
    const { activeTab } = tabs
    if (!activeTab.response || activeTab.response.error || !workspace.workspacePath) return
    const urlPart = activeTab.url.replace(/^https?:\/\//, '').split('?')[0]
      .split('/').filter(Boolean).pop() ?? 'request'
    const defaultName = `${activeTab.method.toLowerCase()}-${urlPart}`.slice(0, 40)
    setNewScenarioName(defaultName)
    setPendingAddToScenario({ defaultName })
  }

  async function confirmAddToScenario() {
    const name = newScenarioName.trim()
    if (!name) return
    setPendingAddToScenario(null)
    const { activeTab } = tabs
    if (!activeTab.response || !workspace.workspacePath) return
    const stepLabel = `${activeTab.method} ${activeTab.url}`.slice(0, 60)
    const scenario: ScenarioDetail = {
      path: '',
      title: name,
      description: '',
      steps: [{
        label: stepLabel,
        method: activeTab.method,
        url: activeTab.url,
        headers: activeTab.headers.filter(h => h.enabled && h.name.trim()).map(h => ({ name: h.name, value: h.value })),
        queryParams: activeTab.queryParams.filter(p => p.enabled && p.name.trim()).map(p => ({ name: p.name, value: p.value })),
        bodyType: activeTab.body.type !== 'none' ? activeTab.body.type : undefined,
        bodyContent: activeTab.body.type !== 'none' ? activeTab.body.content : undefined,
        assertions: [{ type: 'status', operator: 'eq', expected: activeTab.response.statusCode }],
        extractions: [],
      }],
    }
    const yaml = serializeScenarioToYAML(scenario)
    try {
      const result = await createScenario(name, '')
      if (result.error || !result.path) { setAppError(result.error ?? 'Failed to create scenario'); return }
      const saveErr = await saveScenarioRaw(result.path, yaml)
      if (saveErr) { setAppError(saveErr); return }
      await workspace.refreshWorkspace()
      tabs.openScenarioAsTab(result.path, name)
      showScenarios()
    } catch (e) {
      setAppError(String(e).replace(/^Error: /, ''))
    }
  }

  async function handleAddToExistingScenario(scenarioPath: string, scenarioTitle: string) {
    setPendingAddToScenario(null)
    const { activeTab } = tabs
    if (!activeTab.response || !workspace.workspacePath) return
    try {
      const existing = await loadScenario(scenarioPath)
      if (existing.error) { setAppError(existing.error); return }
      const stepLabel = `${activeTab.method} ${activeTab.url}`.slice(0, 60)
      const updated = {
        ...existing,
        steps: [
          ...existing.steps,
          {
            label: stepLabel,
            method: activeTab.method,
            url: activeTab.url,
            headers: activeTab.headers.filter(h => h.enabled && h.name.trim()).map(h => ({ name: h.name, value: h.value })),
            queryParams: activeTab.queryParams.filter(p => p.enabled && p.name.trim()).map(p => ({ name: p.name, value: p.value })),
            bodyType: activeTab.body.type !== 'none' ? activeTab.body.type : undefined,
            bodyContent: activeTab.body.type !== 'none' ? activeTab.body.content : undefined,
            assertions: [{ type: 'status', operator: 'eq', expected: activeTab.response.statusCode }],
            extractions: [],
          },
        ],
      }
      const yaml = serializeScenarioToYAML(updated)
      const saveErr = await saveScenarioRaw(scenarioPath, yaml)
      if (saveErr) { setAppError(saveErr); return }
      tabs.openScenarioAsTab(scenarioPath, scenarioTitle)
      showScenarios()
    } catch (e) {
      setAppError(String(e).replace(/^Error: /, ''))
    }
  }

  return {
    pendingAddToScenario, setPendingAddToScenario, newScenarioName, setNewScenarioName,
    openAddToScenarioDialog, confirmAddToScenario, handleAddToExistingScenario,
  }
}
