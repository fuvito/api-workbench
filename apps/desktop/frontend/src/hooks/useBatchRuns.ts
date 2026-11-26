import { useRef, useState } from 'react'
import { cancelFolderRun, cancelScenario, runFolder, runScenario, saveScenarioResult } from '@/lib/bridge'
import type { FolderRunState, RunAllState } from '@/types'
import type { Tabs, Workspace } from '@/hooks/appTypes'

type Params = {
  tabs: Tabs
  workspace: Workspace
  /** A scenario finished: the run log reads again. */
  onRunLogged: () => void
  /** A batch finished: coverage reads again. */
  onBatchDone: () => void
}

/** Running many at once: all scenarios, a folder of scenarios, or a folder of requests. */
export function useBatchRuns({ tabs, workspace, onRunLogged, onBatchDone }: Params) {
  // Run All Scenarios state
  const [runAllState, setRunAllState] = useState<RunAllState | null>(null)
  const runAllCancelRef = useRef(false)

  // Run all requests in a folder
  const [folderRunState, setFolderRunState] = useState<FolderRunState | null>(null)

  const folderRunStoppedRef = useRef(false)

  function handleCancelFolderRun() {
    folderRunStoppedRef.current = true
    cancelFolderRun().catch(() => {})
  }

  async function handleRunFolder(folderPath: string) {
    const folderName = folderPath.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? folderPath
    folderRunStoppedRef.current = false
    setFolderRunState({ folderPath, folderName, running: true, results: [] })
    try {
      const results = await runFolder(folderPath)
      setFolderRunState({ folderPath, folderName, running: false, stopped: folderRunStoppedRef.current, results })
    } catch (e) {
      setFolderRunState({ folderPath, folderName, running: false, results: [{ name: 'Error', path: '', method: '', statusCode: 0, durationMs: 0, error: String(e), passed: false }] })
    }
  }

  function handleCancelRunAll() {
    runAllCancelRef.current = true
    cancelScenario().catch(() => {})
  }

  async function handleRunScenarioFolder(folderPath: string) {
    const all = workspace.scenarioList.filter(s => !s.isFolder)
    const list = folderPath === ''
      ? all
      : all.filter(s => s.folder === folderPath || s.folder?.startsWith(folderPath + '/'))
    if (!list.length) return
    const envPath = workspace.activeEnv?.path ?? ''
    runAllCancelRef.current = false
    tabs.openRunnerTab()
    setRunAllState({ running: true, done: 0, total: list.length, passed: 0, results: [] })
    let passedCount = 0
    for (const s of list) {
      if (runAllCancelRef.current) break
      try {
        const r = await runScenario(s.path, envPath)
        if (r.passed) passedCount++
        saveScenarioResult(s.path, r).catch(() => {})
        setRunAllState(prev => prev ? {
          ...prev,
          done: prev.done + 1,
          passed: passedCount,
          results: [...prev.results, { path: s.path, name: s.title, passed: r.passed, durationMs: r.durationMs }],
        } : prev)
        onRunLogged()
      } catch (e) {
        setRunAllState(prev => prev ? {
          ...prev,
          done: prev.done + 1,
          results: [...prev.results, { path: s.path, name: s.title, passed: false, durationMs: 0, error: String(e) }],
        } : prev)
      }
    }
    setRunAllState(prev => prev ? { ...prev, running: false } : prev)
    onRunLogged()
    onBatchDone()
  }

  async function handleRunAll() {
    // The list also holds a placeholder per folder (so empty folders can be drawn); only scenarios run.
    const list = workspace.scenarioList.filter(s => !s.isFolder)
    if (!list.length) return
    const envPath = workspace.activeEnv?.path ?? ''
    runAllCancelRef.current = false
    tabs.openRunnerTab()
    setRunAllState({ running: true, done: 0, total: list.length, passed: 0, results: [] })
    let passedCount = 0
    for (const s of list) {
      if (runAllCancelRef.current) break
      try {
        const r = await runScenario(s.path, envPath)
        if (r.passed) passedCount++
        saveScenarioResult(s.path, r).catch(() => {})
        setRunAllState(prev => prev ? {
          ...prev,
          done: prev.done + 1,
          passed: passedCount,
          results: [...prev.results, { path: s.path, name: s.title, passed: r.passed, durationMs: r.durationMs }],
        } : prev)
        onRunLogged()
      } catch (e) {
        setRunAllState(prev => prev ? {
          ...prev,
          done: prev.done + 1,
          results: [...prev.results, { path: s.path, name: s.title, passed: false, durationMs: 0, error: String(e) }],
        } : prev)
      }
    }
    setRunAllState(prev => prev ? { ...prev, running: false } : prev)
    onRunLogged()
    onBatchDone()
  }

  return {
    runAllState, setRunAllState, folderRunState, setFolderRunState,
    handleRunAll, handleRunScenarioFolder, handleCancelRunAll, handleRunFolder, handleCancelFolderRun,
  }
}
