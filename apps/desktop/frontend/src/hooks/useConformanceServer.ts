import { useState } from 'react'
import { openConformanceWorkspace, stopConformanceServer } from '@/lib/bridge'
import { withoutTabsIn } from '@/lib/tabState'
import type { Tabs, Workspace } from '@/hooks/appTypes'

type Params = {
  tabs: Tabs
  workspace: Workspace
  switchToExplorer: () => void
  onError: (message: string) => void
}

export function useConformanceServer({ tabs, workspace, switchToExplorer, onError }: Params) {
  const [conformancePort, setConformancePort] = useState(0)
  const [conformanceWorkspace, setConformanceWorkspace] = useState('')
  const [conformanceStarting, setConformanceStarting] = useState(false)
  // Like the Demo Server: starting it opens its temporary workspace (the spec, a local environment,
  // a sample scenario and requests) and stopping it closes that workspace again.
  async function handleStartConformanceServer() {
    if (conformanceStarting) return
    setConformanceStarting(true)
    try {
      const result = await openConformanceWorkspace()
      if (result.error) { onError(result.error); return }
      setConformancePort(result.port)
      setConformanceWorkspace(result.workspacePath ?? '')
      await workspace.refreshWorkspace()
      switchToExplorer()
      tabs.openWorkspaceTab()
      tabs.openConformanceTab()
    } finally {
      setConformanceStarting(false)
    }
  }
  async function handleStopConformanceServer() {
    const port = conformancePort
    const wsPath = conformanceWorkspace
    await stopConformanceServer()
    setConformancePort(0)
    setConformanceWorkspace('')
    if (wsPath) {
      tabs.setTabsState(prev => withoutTabsIn(prev, port > 0 ? `http://localhost:${port}` : null, wsPath))
      await workspace.refreshWorkspace()
    }
  }

  return { conformancePort, conformanceStarting, handleStartConformanceServer, handleStopConformanceServer }
}
