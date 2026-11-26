import {useEffect, useRef, useState} from 'react'
import {QuickOpen} from '@/components/QuickOpen'
import {MenuBar} from '@/components/MenuBar'
import {TabBar} from '@/components/TabBar'
import {AppToolbar} from '@/components/AppToolbar'
import type {RailTab} from '@/components/ActivityRail'
import {ActivityRail} from '@/components/ActivityRail'
import {StatusBar} from '@/components/StatusBar'
import {LandingPage} from '@/components/LandingPage'
import {SettingsPanel} from '@/components/SettingsPanel'
import {HelpPanel} from '@/components/HelpPanel'
import type {HelpTab} from '@/components/HelpPanel'
import {EnvironmentEditor} from '@/components/EnvironmentEditor'
import {RequestEditor} from '@/components/RequestEditor'
import {ResponseViewer} from '@/components/ResponseViewer'
import {ChevronLeft} from 'lucide-react'
import {ScenarioViewer} from '@/components/ScenarioViewer'
import {CoverageTab} from '@/components/CoverageTab'
import {ConformanceTab} from '@/components/ConformanceTab'
import {WorkspaceTab} from '@/components/WorkspaceTab'
import {WorkspaceSidebarPanel} from '@/components/WorkspaceSidebarPanel'
import {ConformanceSidebarPanel} from '@/components/ConformanceSidebarPanel'
import type {ConformSummary} from '@/components/ConformanceSidebarPanel'
import {CliBuilderSidebarPanel} from '@/components/CliBuilderSidebarPanel'
import type {ConformRequest} from '@/components/ConformanceTab'
import {useWorkspaceOverview} from '@/hooks/useWorkspaceOverview'
import {RunnerTab} from '@/components/RunnerTab'
import {CliBuilderTab} from '@/components/CliBuilderTab'
import type {CliBuilderPreset} from '@/components/CliBuilderTab'
import {WorkspaceImportDialog} from '@/components/WorkspaceImportDialog'
import {BundleImportDialog} from '@/components/BundleImportDialog'
import {ImportReportDialog} from '@/components/ImportReportDialog'
import {Sidebar} from '@/components/Sidebar'
import {AboutDialog} from '@/components/AboutDialog'
import {
  CloseDirtyTabDialog,
  DeleteFolderDialog,
  DeleteRequestDialog,
  DeleteScenarioDialog,
  DiscardChangesDialog,
  ImportFromURLDialog,
  NewFolderDialog,
  ExportSecretsWarningDialog,
  NewScenarioFromResponseDialog,
} from '@/components/ConfirmDialogs'
import {useTabsState} from '@/hooks/useTabsState'
import {useWorkspace} from '@/hooks/useWorkspace'
import {useSettings} from '@/hooks/useSettings'
import {useDemo} from '@/hooks/useDemo'
import type {ScanImportResult, HistoryEntry} from '@/lib/bridge'
import {
    cancelRequest,
    clearResponseCache,
    createScenario,
    deleteScenario,
    executeRequest,
    exportWorkspace,
    exportBundle,
    workspaceHasSecrets,
    getCloseReminder,
    pickFile,
    saveResponseCache,
    scanWorkspaceImport,
    selectEnvironment,
    stopDemo,
    openInExplorer,
    listDynamicFunctions,
} from '@/lib/bridge'
import {setDynamicFunctions} from '@/lib/dynamicFunctions'
import {authStateToPayload} from '@/lib/httpUtils'
import {isTabDirty, isPageTab} from '@/lib/tabState'
import {iconForTab, isRailIcon, recordTab, tabToRestore, type LastTabs} from '@/lib/railIcons'
import {usePanelSizes} from '@/hooks/usePanelSizes'
import {useConformanceServer} from '@/hooks/useConformanceServer'
import {useBatchRuns} from '@/hooks/useBatchRuns'
import {useFileClipboard} from '@/hooks/useFileClipboard'
import {useAddToScenario} from '@/hooks/useAddToScenario'
import {useScenarioFileActions} from '@/hooks/useScenarioFileActions'
import {useAppShortcuts} from '@/hooks/useAppShortcuts'
import {applyUrlPaste} from '@/lib/urlPaste'
import {buildAppMenus} from '@/lib/appMenus'

export default function App() {
  // Cross-dependency ref: useTabsState needs refreshWorkspace before useWorkspace is initialized
  const refreshWorkspaceRef = useRef<() => Promise<void>>(async () => {})

  const tabs = useTabsState({ refreshWorkspaceRef })

  function switchToExplorer() {
    setRailTab('explorer')
    localStorage.setItem('ui.railTab', 'explorer')
    setSidebarOpen(true)
    localStorage.setItem('ui.sidebarOpen', 'true')
  }

  // A workspace the user just opened (open, new, recent, Demo Server) starts on its Workspace tab,
  // with the Workspace sidebar showing.
  function onWorkspaceOpened() {
    setRailTab('workspace')
    localStorage.setItem('ui.railTab', 'workspace')
    setSidebarOpen(true)
    localStorage.setItem('ui.sidebarOpen', 'true')
    tabs.openWorkspaceTab()
  }

  const workspace = useWorkspace({
    setTabsState: tabs.setTabsState,
    tabsStateRef: tabs.tabsStateRef,
    openFileAsTab: tabs.openFileAsTab,
    loadCacheIntoTab: tabs.loadCacheIntoTab,
    onWorkspaceOpened,
  })
  const settings = useSettings()
  const demo = useDemo({
    refreshWorkspace: workspace.refreshWorkspace,
    refreshHistory: workspace.refreshHistory,
    setTabsState: tabs.setTabsState,
    openFirstFileInTree: workspace.openFirstFileInTree,
    onWorkspaceOpened,
    onError: message => setAppError(message),
  })

  const conformance = useConformanceServer({ tabs, workspace, switchToExplorer, onError: message => setAppError(message) })
  const { conformancePort, conformanceStarting, handleStartConformanceServer, handleStopConformanceServer } = conformance

  // Wire cross-dependency after both hooks are initialized
  // eslint-disable-next-line react/refs
  refreshWorkspaceRef.current = workspace.refreshFileTree

  // Window title is set by the Go side via runtime.WindowSetTitle — no React duplicate needed.

  // Layout state
  const { sidebarWidth, responsePct, startDrag } = usePanelSizes()
  const [railTab, setRailTab] = useState<RailTab>(() => {
    const saved = localStorage.getItem('ui.railTab')
    // 'history' was a rail tab of its own before it moved next to the requests list
    return isRailIcon(saved) ? saved : 'explorer'
  })
  const [sidebarOpen, setSidebarOpen] = useState(() => {
    return localStorage.getItem('ui.sidebarOpen') !== 'false'
  })
  // Every rail icon remembers the last tab opened under it (this session only). Switching to a tab
  // lights its icon, points the sidebar at that icon's panel and records the tab; it never opens or
  // closes the sidebar, which is the user's choice. A click on a rail icon (handleRailTabClick) shows
  // the icon's sidebar and brings back its last tab. Not applied on the first render so a saved icon
  // is not overwritten at startup.
  const lastTabs = useRef<LastTabs>({})
  const firstRailRender = useRef(true)
  useEffect(() => {
    lastTabs.current = recordTab(lastTabs.current, tabs.activeTab)
    if (firstRailRender.current) { firstRailRender.current = false; return }
    const icon = iconForTab(tabs.activeTab?.kind)
    setRailTab(icon)
    localStorage.setItem('ui.railTab', icon)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tabs.activeTabId])
  const [railExpanded, setRailExpanded] = useState(() => {
    return localStorage.getItem('ui.railExpanded') === 'true'
  })

  const [appError, setAppError] = useState<string | null>(null)
  const [appNotice, setAppNotice] = useState<string | null>(null)
  const [showQuickOpen, setShowQuickOpen] = useState(false)
  const [responseCollapsed, setResponseCollapsed] = useState(false)

  function openCliBuilder(preset?: { command: string; values?: Record<string, string> }) {
    if (preset) setCliPreset({ id: ++cliPresetSeq.current, ...preset })
    tabs.openCliTab()
  }

  function handleRailTabClick(icon: RailTab) {
    if (icon === railTab && sidebarOpen) {
      setSidebarOpen(false)
      localStorage.setItem('ui.sidebarOpen', 'false')
      return
    }
    setRailTab(icon)
    localStorage.setItem('ui.railTab', icon)
    setSidebarOpen(true)
    localStorage.setItem('ui.sidebarOpen', 'true')
    const id = tabToRestore(lastTabs.current, icon, tabs.tabs)
    if (id) {
      if (id !== tabs.activeTabId) tabs.switchTo(id)
    } else if (icon === 'workspace') tabs.openWorkspaceTab()
    else if (icon === 'conformance') tabs.openConformanceTab()
    else if (icon === 'cli') openCliBuilder()
  }

  function handleToggleRailExpanded() {
    setRailExpanded(prev => {
      const next = !prev
      localStorage.setItem('ui.railExpanded', String(next))
      return next
    })
  }

  // Modal / overlay state
  const [showAbout, setShowAbout] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const [cliPreset, setCliPreset] = useState<CliBuilderPreset | null>(null)
  const cliPresetSeq = useRef(0)
  const [helpInitialTab, setHelpInitialTab] = useState<HelpTab>('guide')
  const [showEnvEditor, setShowEnvEditor] = useState(false)
  const [editingEnvPath, setEditingEnvPath] = useState<string | null>(null)
  const [deletingScenario, setDeletingScenario] = useState<{ path: string; name: string; isDirty: boolean } | null>(null)
  const [pendingCloseWorkspace, setPendingCloseWorkspace] = useState(false)
  const [pendingWorkspaceAction, setPendingWorkspaceAction] = useState<(() => void) | null>(null)

  const { scenarioRenameError, scenarioActions } = useScenarioFileActions({ tabs, workspace, onAskDelete: setDeletingScenario })

  // What closing will remember, appended to the unsaved-changes confirmation.
  const [closeReminder, setCloseReminder] = useState('')
  const [envChanges, setEnvChanges] = useState(0)
  useEffect(() => { setEnvChanges(n => n + 1) }, [workspace.activeEnv?.path])

  async function doCloseWorkspace() {
    if (demo.demoRunning) {
      try { await stopDemo() } catch { /* best-effort */ }
      demo.clearDemoState()
    }
    await workspace.handleCloseWorkspace()
  }

  function requestCloseWorkspace() {
    const hasDirty = tabs.tabs.some(t => isTabDirty(t) || (t.kind === 'scenario' && t.scenarioDirty))
    if (hasDirty) {
      setPendingWorkspaceAction(null)
      getCloseReminder().then(setCloseReminder).catch(() => setCloseReminder(''))
      setPendingCloseWorkspace(true)
    } else {
      doCloseWorkspace()
    }
  }

  function requestWorkspaceSwitch(action: () => void) {
    if (!workspace.workspacePath) { action(); return }
    const hasDirty = tabs.tabs.some(t => isTabDirty(t) || (t.kind === 'scenario' && t.scenarioDirty))
    if (hasDirty) {
      setPendingWorkspaceAction(() => action)
      getCloseReminder().then(setCloseReminder).catch(() => setCloseReminder(''))
      setPendingCloseWorkspace(true)
    } else {
      doCloseWorkspace().then(() => action())
    }
  }

  // Bump after each scenario run to refresh the run log
  const [runLogTrigger, setRunLogTrigger] = useState(0)

  // Which spec is selected in the coverage sidebar (drives the single coverage tab)
  const [selectedCoverageSpecFile, setSelectedCoverageSpecFile] = useState<string | null>(null)
  // Bump to trigger a Contracts tab refresh (on tab switch + after any scenario run)
  const [coverageRefreshTrigger, setCoverageRefreshTrigger] = useState(0)

  // The Workspace page and the Workspace sidebar read one snapshot, so they never disagree. It is read
  // while either is on screen and again when a run, a Contracts change or an environment change happens.
  const workspaceOverview = useWorkspaceOverview({
    workspacePath: workspace.workspacePath,
    enabled: tabs.activeTab?.kind === 'workspace' || (sidebarOpen && railTab === 'workspace'),
    refreshTrigger: runLogTrigger + coverageRefreshTrigger + envChanges,
  })
  // Check Conformance and CLI Builder: the sidebars pick inputs on the pages (a request the page applies),
  // and the pages report back what is chosen so the sidebars can mark it.
  const [conformRequest, setConformRequest] = useState<ConformRequest | null>(null)
  const [conformSel, setConformSel] = useState({ specPath: '', envPath: '' })
  const [conformResult, setConformResult] = useState<ConformSummary | null>(null)
  const [cliCommand, setCliCommand] = useState('')
  function chooseConformInput(choice: { specPath?: string; envPath?: string }) {
    setConformRequest(prev => ({ id: (prev?.id ?? 0) + 1, ...choice }))
    tabs.openConformanceTab()
  }
  const [focusAttention, setFocusAttention] = useState<{ index: number; seq: number }>()
  function showAttention(index: number) {
    tabs.openWorkspaceTab()
    setFocusAttention(prev => ({ index, seq: (prev?.seq ?? 0) + 1 }))
  }

  // Workspace export / import
  const [pendingImport, setPendingImport] = useState<ScanImportResult | null>(null)
  const [showBundleImport, setShowBundleImport] = useState(false)
  const [showExportSecretsWarning, setShowExportSecretsWarning] = useState(false)

  // Prefilled variable name for inline env creation
  const [prefilledVarName, setPrefilledVarName] = useState<string | null>(null)

  async function handleExportWorkspace() {
    const hasSecrets = await workspaceHasSecrets()
    if (hasSecrets) {
      setShowExportSecretsWarning(true)
      return
    }
    const err = await exportWorkspace()
    if (err) setAppError(err)
  }

  async function doExportWorkspace() {
    setShowExportSecretsWarning(false)
    const err = await exportWorkspace()
    if (err) setAppError(err)
  }

  async function handleExportBundle() {
    try {
      const r = await exportBundle()
      if (r.error) { setAppError(r.error); return }
      if (!r.path) return  // cancelled
      const parts = [`${r.environments} environment${r.environments !== 1 ? 's' : ''}`, `${r.requests} request${r.requests !== 1 ? 's' : ''}`, `${r.scenarios} scenario${r.scenarios !== 1 ? 's' : ''}`]
      let msg = `Exported ${parts.join(', ')} to ${r.path}.`
      if (r.redactedSecrets > 0) msg += ` ${r.redactedSecrets} secret value${r.redactedSecrets !== 1 ? 's were' : ' was'} replaced with CHANGE_ME.`
      if (r.skipped.length > 0) msg += ` Skipped ${r.skipped.length} file${r.skipped.length !== 1 ? 's' : ''} that could not be read: ${r.skipped.join('; ')}.`
      if (r.warnings.length > 0) msg += ` ${r.warnings.join(' ')}`
      setAppNotice(msg)
    } catch (e) {
      setAppError(String(e).replace(/^Error: /, ''))
    }
  }

  async function handleImportWorkspace() {
    const scan = await scanWorkspaceImport()
    if (!scan.zipPath) return  // cancelled or error
    if (scan.error) { setAppError(scan.error); return }
    setPendingImport(scan)
  }

  const {
    runAllState, setRunAllState, folderRunState, setFolderRunState,
    handleRunAll, handleRunScenarioFolder, handleCancelRunAll, handleRunFolder, handleCancelFolderRun,
  } = useBatchRuns({
    tabs,
    workspace,
    onRunLogged: () => setRunLogTrigger(t => t + 1),
    onBatchDone: () => setCoverageRefreshTrigger(t => t + 1),
  })
  const { fileClipboard, setFileClipboard, handlePasteToRequestFolder, handlePasteToScenarioFolder, handleMoveScenario } =
    useFileClipboard({ tabs, workspace, onError: setAppError })

  // Init
  useEffect(() => {
    workspace.refreshWorkspace()
    workspace.refreshHistory()
    settings.loadSettings()
    Promise.resolve(listDynamicFunctions()).then(list => setDynamicFunctions(list ?? [])).catch(() => {})
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Request execution
  const handleSendRef = useRef<(() => void) | null>(null)

  async function handleClearResponse() {
    const { activeTab, activeTabId, updateTab } = tabs
    updateTab(activeTabId, { response: null, responseCachedAt: null })
    if (activeTab.filePath) {
      clearResponseCache(activeTab.filePath).catch(() => {})
    }
  }

  async function handleSend() {
    const { activeTab, activeTabId, updateTab, tabsStateRef } = tabs
    if (!activeTab.url.trim() || activeTab.loading) return
    const tabId = activeTabId
    const prevResponse = activeTab.response
    updateTab(tabId, { loading: true, response: null, responseCachedAt: null, previousResponse: prevResponse })
    try {
      const result = await executeRequest({
        method: activeTab.method,
        url: activeTab.url.trim(),
        headers: activeTab.headers.filter(h => h.enabled && h.name.trim()).map(h => ({ name: h.name, value: h.value })),
        queryParams: activeTab.queryParams.filter(p => p.enabled && p.name.trim()).map(p => ({ name: p.name, value: p.value })),
        body: activeTab.body.type === 'none' ? undefined : { type: activeTab.body.type, content: activeTab.body.content },
        auth: authStateToPayload(activeTab.auth),
        timeoutMs: activeTab.timeoutMs,
        noFollowRedirects: activeTab.noFollowRedirects || undefined,
      })
      updateTab(tabId, { response: result })
      const filePath = tabsStateRef.current.tabs.find(t => t.id === tabId)?.filePath
      if (filePath && !result.error) {
        saveResponseCache(filePath, result).catch(() => {})
      }
    } catch (err) {
      updateTab(tabId, { response: { statusCode: 0, status: '', headers: [], body: '', durationMs: 0, size: 0, timing: { dnsMs: 0, connectMs: 0, tlsMs: 0, ttfbMs: 0, transferMs: 0 }, error: String(err) } })
    } finally {
      updateTab(tabId, { loading: false })
      workspace.refreshHistory()
    }
  }

  // eslint-disable-next-line react/refs
  handleSendRef.current = handleSend

  const workspacePathRef = useRef(workspace.workspacePath)
  // eslint-disable-next-line react/refs
  workspacePathRef.current = workspace.workspacePath

  function handleCancel() {
    cancelRequest().catch(() => {})
  }

  function handleRerunHistory(entry: HistoryEntry) {
    tabs.handleSelectHistory(entry)
    setTimeout(() => handleSendRef.current?.(), 0)
  }

  // The Scenarios sidebar, open, as after making or opening a scenario from elsewhere
  function showScenarios() {
    setRailTab('scenarios')
    setSidebarOpen(true)
    localStorage.setItem('ui.railTab', 'scenarios')
    localStorage.setItem('ui.sidebarOpen', 'true')
  }

  const {
    pendingAddToScenario, setPendingAddToScenario, newScenarioName, setNewScenarioName,
    openAddToScenarioDialog, confirmAddToScenario, handleAddToExistingScenario,
  } = useAddToScenario({ tabs, workspace, showScenarios, onError: setAppError })

  function handleCreateVariableFromHint(name: string) {
    setPrefilledVarName(name)
    setEditingEnvPath(workspace.activeEnv?.path ?? null)
    setShowEnvEditor(true)
  }

  useAppShortcuts({ tabs, handleSendRef, workspacePathRef, setSidebarOpen, setShowQuickOpen })

  const menus = buildAppMenus({
    tabs, workspace, settings, demo,
    conformancePort, conformanceStarting, handleStartConformanceServer, handleStopConformanceServer,
    requestWorkspaceSwitch, requestCloseWorkspace,
    handleExportWorkspace, handleImportWorkspace, handleExportBundle,
    showHelp: tab => { setHelpInitialTab(tab); setShowHelp(true) },
    showAbout: () => setShowAbout(true),
    showBundleImport: () => setShowBundleImport(true),
  })

  const { tabs: tabList, activeTabId, activeTab, updateTab } = tabs
  const closingTab = tabs.closingDirtyTabId ? tabList.find(t => t.id === tabs.closingDirtyTabId) : null
  const tabInfos = tabList.map(t => ({
    id: t.id,
    name: t.name,
    method: t.method,
    isDirty: isTabDirty(t),
    kind: (t.kind ?? 'request') as 'request' | 'scenario' | 'coverage' | 'runner' | 'cli' | 'conformance' | 'workspace',
    filePath: t.kind === 'scenario' ? (t.scenarioPath ?? null) : (t.filePath ?? null),
  }))

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden">
      <MenuBar menus={menus} />

      {appError && (
        <div className="shrink-0 px-4 py-2 bg-red-500/15 border-b border-red-500/30 flex items-center gap-2 text-sm text-red-400">
          <span className="flex-1">{appError}</span>
          <button type="button" onClick={() => setAppError(null)} className="text-red-400/60 hover:text-red-400 transition-colors text-xs">Dismiss</button>
        </div>
      )}

      {appNotice && (
        <div role="status" className="shrink-0 px-4 py-2 bg-emerald-500/15 border-b border-emerald-500/30 flex items-center gap-2 text-sm text-emerald-400">
          <span className="flex-1">{appNotice}</span>
          <button type="button" onClick={() => setAppNotice(null)} className="text-emerald-400/60 hover:text-emerald-400 transition-colors text-xs">Dismiss</button>
        </div>
      )}

      {/* Dialogs */}
      {showExportSecretsWarning && (
        <ExportSecretsWarningDialog
          onCancel={() => setShowExportSecretsWarning(false)}
          onConfirm={doExportWorkspace}
        />
      )}
      {pendingAddToScenario && (
        <NewScenarioFromResponseDialog
          defaultName={pendingAddToScenario.defaultName}
          value={newScenarioName}
          onChange={setNewScenarioName}
          onCancel={() => setPendingAddToScenario(null)}
          onConfirm={confirmAddToScenario}
          scenarios={workspace.scenarioList.filter(s => !s.isFolder)}
          onAddToExisting={handleAddToExistingScenario}
        />
      )}
      {workspace.deletingFilePath && (
        <DeleteRequestDialog
          filePath={workspace.deletingFilePath}
          onCancel={() => workspace.setDeletingFilePath(null)}
          onConfirm={workspace.confirmDeleteFile}
        />
      )}
      {showAbout && <AboutDialog onClose={() => setShowAbout(false)} />}
      {showHelp && <HelpPanel onClose={() => setShowHelp(false)} initialTab={helpInitialTab} />}
      {workspace.importReport && (
        <ImportReportDialog report={workspace.importReport} onClose={() => workspace.setImportReport(null)} />
      )}

      {workspace.importFromURLDialog && (
        <ImportFromURLDialog
          value={workspace.importFromURLInput}
          onChange={v => { workspace.setImportFromURLInput(v); workspace.setImportFromURLError('') }}
          onCancel={() => { workspace.setImportFromURLDialog(false); workspace.setImportFromURLError('') }}
          onConfirm={workspace.handleImportFromURL}
          error={workspace.importFromURLError}
        />
      )}
      {workspace.newFolderDialog && (
        <NewFolderDialog
          parentLabel={workspace.newFolderDialog.parentLabel}
          value={workspace.newFolderName}
          onChange={v => { workspace.setNewFolderName(v); workspace.setNewFolderError('') }}
          onCancel={() => { workspace.setNewFolderDialog(null); workspace.setNewFolderError('') }}
          onConfirm={workspace.confirmCreateFolder}
          error={workspace.newFolderError}
        />
      )}
      {workspace.deletingFolderPath && (
        <DeleteFolderDialog
          folderPath={workspace.deletingFolderPath}
          onCancel={() => workspace.setDeletingFolderPath(null)}
          onConfirm={workspace.confirmDeleteFolder}
        />
      )}
      {closingTab && (
        <CloseDirtyTabDialog
          tabName={closingTab.name}
          onCancel={() => tabs.setClosingDirtyTabId(null)}
          onDiscard={() => { tabs.setClosingDirtyTabId(null); tabs.removeTab(closingTab.id) }}
          onSave={async () => {
            const ok = await tabs.saveTabById(closingTab.id)
            if (ok) { tabs.setClosingDirtyTabId(null); tabs.removeTab(closingTab.id) }
          }}
        />
      )}
      {tabs.closingDirtyScenarioTabId && (() => {
        const t = tabList.find(x => x.id === tabs.closingDirtyScenarioTabId)
        return t ? (
          <DiscardChangesDialog
            title="Close scenario"
            message={`"${t.name ?? 'This scenario'}" has unsaved changes. Close and discard them?`}
            onCancel={() => tabs.setClosingDirtyScenarioTabId(null)}
            onDiscard={() => { tabs.removeTab(t.id); tabs.setClosingDirtyScenarioTabId(null) }}
          />
        ) : null
      })()}
      {tabs.pendingBulkClose && (() => {
        const dirtyCount = tabs.pendingBulkClose.tabIds.filter(id => {
          const t = tabList.find(x => x.id === id)
          return t && (isTabDirty(t) || (t.kind === 'scenario' && t.scenarioDirty))
        }).length
        const noun = dirtyCount === 1 ? 'tab has' : 'tabs have'
        return (
          <DiscardChangesDialog
            title="Close tabs"
            message={`${dirtyCount} ${noun} unsaved changes. Close and discard them?`}
            onCancel={() => tabs.setPendingBulkClose(null)}
            onDiscard={tabs.confirmBulkClose}
          />
        )
      })()}
      {pendingCloseWorkspace && (() => {
        const dirtyCount = tabList.filter(t => isTabDirty(t) || (t.kind === 'scenario' && t.scenarioDirty)).length
        const noun = dirtyCount === 1 ? 'tab has' : 'tabs have'
        const action = pendingWorkspaceAction
        return (
          <DiscardChangesDialog
            title={action ? 'Switch workspace' : 'Close workspace'}
            message={`${dirtyCount} ${noun} unsaved changes. ${action ? 'Switch workspace' : 'Close workspace'} and discard them?${closeReminder ? ` ${closeReminder}` : ''}`}
            onCancel={() => { setPendingCloseWorkspace(false); setPendingWorkspaceAction(null) }}
            onDiscard={() => {
              setPendingCloseWorkspace(false)
              setPendingWorkspaceAction(null)
              doCloseWorkspace().then(() => action?.())
            }}
          />
        )
      })()}
      {deletingScenario && (
        <DeleteScenarioDialog
          name={deletingScenario.name}
          isDirty={deletingScenario.isDirty}
          onCancel={() => setDeletingScenario(null)}
          onConfirm={async () => {
            const { path } = deletingScenario
            setDeletingScenario(null)
            const matchTab = tabList.find(t => t.kind === 'scenario' && t.scenarioPath === path)
            const err = await deleteScenario(path)
            if (!err) {
              if (matchTab) tabs.removeTab(matchTab.id)
              await workspace.refreshWorkspace()
            }
          }}
        />
      )}

      {showBundleImport && (
        <BundleImportDialog
          onClose={() => setShowBundleImport(false)}
          onComplete={() => { workspace.refreshWorkspace() }}
        />
      )}

      {pendingImport && (
        <WorkspaceImportDialog
          scan={pendingImport}
          onClose={() => setPendingImport(null)}
          onComplete={() => { workspace.refreshWorkspace(); setPendingImport(null) }}
        />
      )}

      {showQuickOpen && (
        <QuickOpen
          fileTree={workspace.fileTree}
          scenarios={workspace.scenarioList}
          environments={workspace.environments}
          onSelect={workspace.handleSelectFile}
          onSelectScenario={(path, name) => tabs.openScenarioAsTab(path, name)}
          onSelectEnvironment={path => { workspace.handleEnvChange(path); setShowQuickOpen(false) }}
          onNewRequest={() => { tabs.handleNewTab(); setShowQuickOpen(false) }}
          onNewScenario={async () => {
            setShowQuickOpen(false)
            const result = await createScenario('New Scenario', '')
            if (result.error || !result.path) { setAppError(result.error ?? 'Failed to create scenario'); return }
            await workspace.refreshWorkspace()
            tabs.openScenarioAsTab(result.path, 'New Scenario')
            showScenarios()
          }}
          onClose={() => setShowQuickOpen(false)}
        />
      )}

      {/* Overlays */}
      {settings.showSettings && (
        <SettingsPanel
          settings={settings.settings}
          onSave={settings.handleSaveSettings}
          onReset={settings.handleResetSettings}
          onPreview={settings.applySettings}
          onClose={() => { settings.applySettings(settings.settings); settings.setShowSettings(false) }}
        />
      )}
      {showEnvEditor && (
        <EnvironmentEditor
          envPath={editingEnvPath}
          workspacePath={workspace.workspacePath}
          prefilledVarName={prefilledVarName ?? undefined}
          onClose={() => { setShowEnvEditor(false); setPrefilledVarName(null) }}
          onSaved={async saved => {
            setShowEnvEditor(false)
            setPrefilledVarName(null)
            await workspace.refreshWorkspace()
            try { await selectEnvironment(saved.path) } catch { /* best-effort */ }
            await workspace.refreshWorkspace()
          }}
          onDeleted={async () => {
            setShowEnvEditor(false)
            setPrefilledVarName(null)
            await workspace.refreshWorkspace()
          }}
        />
      )}

      {/* Main area — gated on workspace */}
      {workspace.workspacePath ? (
        <>
          <div className="flex flex-1 overflow-hidden">
            <ActivityRail
              activeTab={railTab}
              expanded={railExpanded}
              onTabClick={handleRailTabClick}
              onToggleExpanded={handleToggleRailExpanded}
              onShowSettings={() => settings.setShowSettings(true)}
              onShowHelp={() => setShowHelp(true)}
              demoRunning={demo.demoRunning}
              onStopDemo={demo.handleStopDemo}
            />

            {sidebarOpen && (
              <>
                <aside
                  data-testid="sidebar"
                  className="shrink-0 flex flex-col border-r border-[var(--color-border)] bg-[var(--color-card)] overflow-hidden"
                  style={{ width: sidebarWidth }}
                >
                  <Sidebar
                    tab={railTab}
                    workspacePath={workspace.workspacePath}
                    fileTree={workspace.fileTree}
                    history={workspace.history}
                    onOpenWorkspace={workspace.handleOpenWorkspace}
                    onNewWorkspace={workspace.handleNewWorkspace}
                    onSelectFile={workspace.handleSelectFile}
                    onDeleteFile={workspace.handleDeleteFile}
                    onDuplicateFile={workspace.handleDuplicateFile}
                    onDeleteFolder={workspace.handleDeleteFolder}
                    onNewRequestInFolder={tabs.handleNewRequestInFolder}
                    onCreateFolder={workspace.handleCreateFolder}
                    onRenameFile={workspace.handleRenameFile}
                    onRenameFolder={workspace.handleRenameFolder}
                    onMoveFile={workspace.handleMoveFile}
                    onSelectHistory={tabs.handleSelectHistory}
                    onRerun={handleRerunHistory}
                    onClearHistory={workspace.handleClearHistory}
                    onImportOpenAPI={workspace.handleImportOpenAPI}
                    onImportPostman={workspace.handleImportPostman}
                    onImportInsomnia={workspace.handleImportInsomnia}
                    onTryDemo={() => requestWorkspaceSwitch(demo.handleStartDemo)}
                    demoStarting={demo.demoStarting}
                    onRefresh={workspace.refreshWorkspace}
                    importing={workspace.importing}
                    importError={workspace.importError}
                    activeFilePath={activeTab.filePath ?? undefined}
                    dirtyFilePaths={tabs.dirtyFilePaths}
                    scenarios={workspace.scenarioList}
                    onSelectScenario={tabs.openScenarioAsTab}
                    scenarioError={scenarioRenameError}
                    runAllState={runAllState ?? undefined}
                    onRunAllScenarios={handleRunAll}
                    runLogTrigger={runLogTrigger}
                    activeScenarioPath={activeTab.kind === 'scenario' ? activeTab.scenarioPath : null}
                    dirtyScenarioPaths={new Set(tabs.tabs.filter(t => t.kind === 'scenario' && t.scenarioDirty && t.scenarioPath).map(t => t.scenarioPath!))}
                    onRunFolder={handleRunFolder}
                    onRunScenarioFolder={handleRunScenarioFolder}
                    folderRunState={folderRunState}
                    onDismissFolderRun={() => setFolderRunState(null)}
                    onCancelFolderRun={handleCancelFolderRun}
                    selectedCoverageSpecFile={selectedCoverageSpecFile}
                    onSelectCoverageSpec={specFile => {
                      setSelectedCoverageSpecFile(specFile)
                      tabs.openCoverageTab()
                      setCoverageRefreshTrigger(t => t + 1)
                    }}
                    workspacePanel={
                      <WorkspaceSidebarPanel
                        overview={workspaceOverview.overview}
                        error={workspaceOverview.error}
                        loading={workspaceOverview.loading}
                        onRefresh={() => void workspaceOverview.reload()}
                        environments={workspace.environments}
                        activeEnvPath={workspace.activeEnv?.path ?? ''}
                        onEnvChange={workspace.handleEnvChange}
                        onShowAttention={showAttention}
                        onShowIcon={handleRailTabClick}
                      />
                    }
                    conformancePanel={
                      <ConformanceSidebarPanel
                        workspacePath={workspace.workspacePath}
                        selectedSpec={conformSel.specPath}
                        selectedEnv={conformSel.envPath}
                        lastResult={conformResult}
                        onSelectSpec={path => chooseConformInput({ specPath: path })}
                        onSelectEnv={path => chooseConformInput({ envPath: path })}
                      />
                    }
                    cliPanel={<CliBuilderSidebarPanel selected={cliCommand} onSelect={name => openCliBuilder({ command: name })} />}
                    clipboard={fileClipboard}
                    onCopyFile={path => setFileClipboard({ op: 'copy', kind: 'request', srcPath: path })}
                    onCutFile={path => setFileClipboard({ op: 'cut', kind: 'request', srcPath: path })}
                    onPasteToRequestFolder={handlePasteToRequestFolder}
                    onPasteToRequestRoot={() => handlePasteToRequestFolder(`${workspace.workspacePath.replace(/\\/g, '/')}/requests`)}
                    onMoveScenario={handleMoveScenario}
                    onCopyScenario={path => setFileClipboard({ op: 'copy', kind: 'scenario', srcPath: path })}
                    onCutScenario={path => setFileClipboard({ op: 'cut', kind: 'scenario', srcPath: path })}
                    onPasteToScenarioFolder={handlePasteToScenarioFolder}
                    {...scenarioActions}
                  />
                </aside>

                <div
                  className="w-1 shrink-0 cursor-col-resize hover:bg-[var(--color-accent)] transition-colors"
                  onMouseDown={e => startDrag('sidebar', e)}
                />
              </>
            )}

            <main className="flex-1 flex flex-col overflow-hidden min-w-0">
              <TabBar
                tabs={tabInfos}
                activeTabId={activeTabId}
                onSelect={tabs.handleSelectTab}
                onClose={tabs.handleCloseTab}
                onNew={tabs.handleNewTab}
                onSave={tabs.saveTabById}
                onCloseOthers={tabs.handleCloseOthers}
                onCloseToRight={tabs.handleCloseToRight}
                onCloseAll={tabs.handleCloseAll}
                onCopyPath={p => navigator.clipboard.writeText(p).catch(() => {})}
                onRevealInExplorer={p => openInExplorer(p).catch(() => {})}
              />

              {!isPageTab(activeTab.kind) && (
                <AppToolbar
                  activeTab={activeTab}
                  activeTabId={activeTabId}
                  updateTab={updateTab}
                  onOpenRequest={workspace.handleOpenRequest}
                  onSaveRequest={tabs.handleSaveRequest}
                  onNewRequest={tabs.handleNewTab}
                  onDuplicateTab={tabs.handleDuplicateTab}
                />
              )}

              <div className="flex-1 flex overflow-hidden relative">
                {/* Runner tab (single instance) */}
                {tabList.filter(t => t.kind === 'runner').map(t => (
                  <div
                    key={t.id}
                    className="absolute inset-0 overflow-hidden"
                    style={{ display: t.id === activeTabId ? 'flex' : 'none', flexDirection: 'column' }}
                  >
                    <RunnerTab
                      runAllState={runAllState}
                      onSelectScenario={tabs.openScenarioAsTab}
                      onCancel={handleCancelRunAll}
                      onDismiss={() => setRunAllState(null)}
                      onRunAll={handleRunAll}
                    />
                  </div>
                ))}

                {/* Conformance tab (single instance) */}
                {tabList.filter(t => t.kind === 'conformance').map(t => (
                  <div
                    key={t.id}
                    className="absolute inset-0 overflow-hidden"
                    style={{ display: t.id === activeTabId ? 'flex' : 'none', flexDirection: 'column' }}
                  >
                    <ConformanceTab
                      onOpenCliBuilder={values => openCliBuilder({ command: 'conform', values })}
                      request={conformRequest}
                      onSelectionChange={setConformSel}
                      onResult={setConformResult}
                    />
                  </div>
                ))}

                {/* Workspace tab (single instance) */}
                {tabList.filter(t => t.kind === 'workspace').map(t => (
                  <div
                    key={t.id}
                    className="absolute inset-0 overflow-hidden"
                    style={{ display: t.id === activeTabId ? 'flex' : 'none', flexDirection: 'column' }}
                  >
                    <WorkspaceTab
                      overview={workspaceOverview.overview}
                      error={workspaceOverview.error}
                      loading={workspaceOverview.loading}
                      onRefresh={() => void workspaceOverview.reload()}
                      focusAttention={focusAttention}
                      onOpenScenario={(path, name) => tabs.openScenarioAsTab(path, name)}
                      onOpenContracts={specRef => {
                        setSelectedCoverageSpecFile(specRef)
                        tabs.openCoverageTab()
                        setCoverageRefreshTrigger(n => n + 1)
                      }}
                      onShowPanel={handleRailTabClick}
                      onCloseWorkspace={requestCloseWorkspace}
                    />
                  </div>
                ))}

                {/* CLI Builder tab (single instance) */}
                {tabList.filter(t => t.kind === 'cli').map(t => (
                  <div
                    key={t.id}
                    className="absolute inset-0 overflow-hidden"
                    style={{ display: t.id === activeTabId ? 'flex' : 'none', flexDirection: 'column' }}
                  >
                    <CliBuilderTab
                      preset={cliPreset}
                      onCommandChange={setCliCommand}
                      runningServers={[
                        ...(demo.demoRunning && demo.demoPort > 0 ? [{ label: 'Demo Server', url: `http://localhost:${demo.demoPort}` }] : []),
                        ...(conformancePort > 0 ? [{ label: 'Conformance Server', url: `http://localhost:${conformancePort}` }] : []),
                      ]}
                    />
                  </div>
                ))}

                {/* Coverage tab (single instance) */}
                {tabList.filter(t => t.kind === 'coverage').map(t => (
                  <div
                    key={t.id}
                    className="absolute inset-0 overflow-hidden"
                    style={{ display: t.id === activeTabId ? 'flex' : 'none', flexDirection: 'column' }}
                  >
                    <CoverageTab
                      workspacePath={workspace.workspacePath}
                      specFile={selectedCoverageSpecFile}
                      refreshTrigger={coverageRefreshTrigger}
                    />
                  </div>
                ))}

                {/* Scenario viewers: keep mounted once opened so edit state survives tab switches */}
                {tabList.filter(t => t.kind === 'scenario').map(t => (
                  <div
                    key={t.id}
                    className="absolute inset-0 overflow-hidden"
                    style={{ display: t.id === activeTabId ? 'flex' : 'none' }}
                  >
                    <ScenarioViewer
                      scenarioPath={t.scenarioPath ?? ''}
                      scenarioName={t.scenarioName ?? ''}
                      result={t.scenarioResult ?? null}
                      running={t.scenarioRunning ?? false}
                      isActive={t.id === activeTabId}
                      saveSignal={t.scenarioSaveSignal}
                      theme={settings.settings.theme}
                      environments={workspace.environments}
                      activeEnvPath={workspace.activeEnv?.path ?? ''}
                      onResultChange={r => updateTab(t.id, { scenarioResult: r })}
                      onRunningChange={running => updateTab(t.id, { scenarioRunning: running })}
                      onDirtyChange={dirty => updateTab(t.id, { scenarioDirty: dirty })}
                      onRunComplete={() => { setRunLogTrigger(t => t + 1); setCoverageRefreshTrigger(t => t + 1) }}
                      onGetCliCommand={() => openCliBuilder({ command: 'run', values: { 'scenario-file': t.scenarioPath ?? '' } })}
                    />
                  </div>
                ))}

                {/* Request panel: only mount when active tab is a request */}
                {activeTab.kind !== 'scenario' && !isPageTab(activeTab.kind) && (
                  <>
                    <div className="flex-1 flex flex-col overflow-hidden min-w-0">
                      <RequestEditor
                        method={activeTab.method}
                        url={activeTab.url}
                        loading={activeTab.loading}
                        queryParams={activeTab.queryParams}
                        headers={activeTab.headers}
                        body={activeTab.body}
                        auth={activeTab.auth}
                        onMethodChange={m => updateTab(activeTabId, { method: m })}
                        onUrlChange={u => updateTab(activeTabId, { url: u })}
                        onUrlPaste={raw => applyUrlPaste(raw, activeTab, updateTab)}
                        onQueryParamsChange={p => updateTab(activeTabId, { queryParams: p })}
                        onHeadersChange={h => updateTab(activeTabId, { headers: h })}
                        onBodyChange={b => updateTab(activeTabId, { body: b })}
                        onAuthChange={a => updateTab(activeTabId, { auth: a })}
                        onPickFile={pickFile}
                        timeoutMs={activeTab.timeoutMs}
                        onTimeoutChange={ms => updateTab(activeTabId, { timeoutMs: ms })}
                        noFollowRedirects={activeTab.noFollowRedirects ?? false}
                        onNoFollowRedirectsChange={v => updateTab(activeTabId, { noFollowRedirects: v })}
                        envVarMap={workspace.envVarMap}
                        onSend={handleSend}
                        onCancel={handleCancel}
                        onCreateVariable={handleCreateVariableFromHint}
                      />
                    </div>

                    {responseCollapsed ? (
                      <div
                        className="shrink-0 w-7 border-l border-[var(--color-border)] bg-[var(--color-card)] flex flex-col items-center justify-start pt-2 cursor-pointer hover:bg-[var(--color-muted)] transition-colors"
                        onClick={() => setResponseCollapsed(false)}
                        title="Expand response panel"
                      >
                        <ChevronLeft size={12} className="text-[var(--color-muted-foreground)]" />
                      </div>
                    ) : (
                      <>
                        <div
                          className="w-1 shrink-0 cursor-col-resize hover:bg-[var(--color-accent)] transition-colors"
                          onMouseDown={e => startDrag('response', e)}
                        />

                        <div
                          className="flex flex-col overflow-hidden min-w-0 shrink-0"
                          style={{ width: `${responsePct}%` }}
                        >
                          <ResponseViewer response={activeTab.response} previousResponse={activeTab.previousResponse} loading={activeTab.loading} cachedAt={activeTab.responseCachedAt} onClear={handleClearResponse} onCollapse={() => setResponseCollapsed(true)} onAddToScenario={workspace.workspacePath ? openAddToScenarioDialog : undefined} />
                        </div>
                      </>
                    )}
                  </>
                )}
              </div>
            </main>
          </div>

          <StatusBar
            environments={workspace.environments}
            activeEnv={workspace.activeEnv}
            workspacePath={workspace.workspacePath}
            activeFilePath={
              activeTab.kind === 'scenario' ? activeTab.scenarioPath
                : isPageTab(activeTab.kind) ? null
                : activeTab.filePath
            }
            demoRunning={demo.demoRunning}
            demoPort={demo.demoPort}
            conformancePort={conformancePort}
            onStopConformance={handleStopConformanceServer}
            onEnvChange={workspace.handleEnvChange}
            onEditEnv={() => { setEditingEnvPath(workspace.activeEnv?.path ?? null); setShowEnvEditor(true) }}
            onStopDemo={demo.handleStopDemo}
          />
        </>
      ) : (
        <LandingPage
          recentWorkspaces={workspace.recentWorkspaces}
          demoStarting={demo.demoStarting}
          demoRunning={demo.demoRunning}
          onNewWorkspace={workspace.handleNewWorkspace}
          onOpenWorkspace={workspace.handleOpenWorkspace}
          onOpenRecentWorkspace={workspace.handleOpenWorkspaceByPath}
          onTryDemo={() => requestWorkspaceSwitch(demo.handleStartDemo)}
          onStopDemo={demo.handleStopDemo}
        />
      )}
    </div>
  )
}
