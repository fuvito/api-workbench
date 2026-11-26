import type { MenuDef } from '@/components/MenuBar'
import type { HelpTab } from '@/components/HelpPanel'
import { openConformanceSpec, openDemoSpec, quitApp, saveContentToFile } from '@/lib/bridge'
import type { Demo, Settings, Tabs, Workspace } from '@/hooks/appTypes'

export type AppMenuContext = {
  tabs: Tabs
  workspace: Workspace
  settings: Settings
  demo: Demo
  conformancePort: number
  conformanceStarting: boolean
  handleStartConformanceServer: () => Promise<void>
  handleStopConformanceServer: () => Promise<void>
  requestWorkspaceSwitch: (action: () => void) => void
  requestCloseWorkspace: () => void
  handleExportWorkspace: () => void
  handleImportWorkspace: () => void
  handleExportBundle: () => void
  showHelp: (tab: HelpTab) => void
  showAbout: () => void
  showBundleImport: () => void
}

export function buildAppMenus({
  tabs, workspace, settings, demo,
  conformancePort, conformanceStarting, handleStartConformanceServer, handleStopConformanceServer,
  requestWorkspaceSwitch, requestCloseWorkspace,
  handleExportWorkspace, handleImportWorkspace, handleExportBundle,
  showHelp, showAbout, showBundleImport,
}: AppMenuContext): MenuDef[] {
  // Menus — context-sensitive: landing (no workspace) vs full (workspace open)
  const helpMenu: MenuDef = {
    label: 'Help',
    items: [
      {
        kind: 'submenu',
        label: 'Dev Servers',
        items: [
          {
            kind: 'action',
            label: demo.demoStarting ? 'Starting…' : demo.demoRunning ? 'Stop Demo Server' : 'Start Demo Server',
            onClick: demo.demoRunning ? demo.handleStopDemo : () => requestWorkspaceSwitch(demo.handleStartDemo),
            disabled: demo.demoStarting,
          },
          {
            kind: 'action',
            label: 'Demo Server API Spec',
            disabled: !demo.demoRunning,
            title: demo.demoRunning ? 'Open the Swagger UI for the demo server in your browser' : 'Start the demo server first',
            onClick: () => openDemoSpec(),
          },
          { kind: 'separator' },
          {
            kind: 'action',
            label: conformanceStarting ? 'Starting…' : conformancePort > 0 ? 'Stop Conformance Server' : 'Start Conformance Server',
            disabled: conformanceStarting || (conformancePort === 0 && demo.demoRunning),
            title: conformancePort === 0 && demo.demoRunning
              ? 'Stop the Demo Server first: its workspace is open'
              : 'A deliberately imperfect API (six operations break its spec). Starting it opens a workspace with its spec, a local environment and a sample scenario',
            onClick: conformancePort > 0 ? handleStopConformanceServer : () => requestWorkspaceSwitch(handleStartConformanceServer),
          },
          {
            kind: 'action',
            label: 'Conformance Server API Spec',
            disabled: conformancePort === 0,
            title: conformancePort > 0 ? 'Open the Swagger UI for the conformance server in your browser' : 'Start the conformance server first',
            onClick: () => openConformanceSpec(),
          },
        ],
      },
      { kind: 'separator' },
      { kind: 'action', label: 'User Guide',   onClick: () => showHelp('guide') },
      { kind: 'action', label: 'Format Spec…', onClick: () => showHelp('spec') },
      { kind: 'action', label: 'Example Scenario…', onClick: () => showHelp('example') },
      { kind: 'action', label: 'AI Prompt…', onClick: () => showHelp('prompt') },
      { kind: 'action', label: 'What We Check…', onClick: () => showHelp('check') },
      { kind: 'separator' },
      { kind: 'action', label: 'About Clear Signal', onClick: showAbout },
    ],
  }
  const viewMenu: MenuDef = {
    label: 'View',
    items: [
      { kind: 'action', label: settings.settings.theme === 'dark' ? 'Switch to Light Theme' : 'Switch to Dark Theme', onClick: settings.handleToggleTheme },
      { kind: 'separator' },
      { kind: 'action', label: 'Settings…', onClick: () => settings.setShowSettings(true) },
    ],
  }
  const recentWorkspacesSubmenu = {
    kind: 'submenu' as const,
    label: 'Recent Workspaces',
    disabled: workspace.recentWorkspaces.length === 0,
    items: workspace.recentWorkspaces.map(p => ({
      kind: 'action' as const,
      label: p.split(/[\\/]/).pop() ?? p,
      title: p,
      onClick: () => requestWorkspaceSwitch(() => workspace.handleOpenWorkspaceByPath(p)),
    })),
  }

  return workspace.workspacePath ? [
    {
      label: 'File',
      items: [
        { kind: 'action', label: 'Open Workspace…',  onClick: () => requestWorkspaceSwitch(workspace.handleOpenWorkspace) },
        { kind: 'action', label: 'New Workspace…',   onClick: () => requestWorkspaceSwitch(workspace.handleNewWorkspace) },
        { kind: 'action', label: 'Close Workspace',  onClick: requestCloseWorkspace },
        { kind: 'separator' },
        { kind: 'action', label: 'Export Workspace…', onClick: handleExportWorkspace },
        { kind: 'action', label: 'Import Workspace…', onClick: handleImportWorkspace },
        recentWorkspacesSubmenu,
        { kind: 'separator' },
        { kind: 'action', label: 'New Tab',       shortcut: 'Ctrl+N',        onClick: tabs.handleNewTab },
        { kind: 'action', label: 'Save Request',  shortcut: 'Ctrl+S',        onClick: tabs.handleSaveRequest },
        { kind: 'action', label: 'Save All',      shortcut: 'Ctrl+Shift+S',  onClick: tabs.handleSaveAll, disabled: tabs.dirtyFilePaths.size === 0 },
        { kind: 'action', label: 'Save As…',                                 onClick: tabs.handleSaveAsRequest },
        { kind: 'separator' },
        {
          kind: 'submenu',
          label: 'Import',
          items: [
            { kind: 'action', label: 'OpenAPI Spec…',       onClick: workspace.handleImportOpenAPI },
            { kind: 'action', label: 'OpenAPI from URL…',   onClick: () => { workspace.setImportFromURLInput(''); workspace.setImportFromURLError(''); workspace.setImportFromURLDialog(true) } },
            { kind: 'separator' },
            { kind: 'action', label: 'Bundle (environments + scenarios)…', onClick: showBundleImport },
            { kind: 'separator' },
            { kind: 'action', label: 'Postman Collection…', onClick: workspace.handleImportPostman },
            { kind: 'action', label: 'Insomnia Export…',    onClick: workspace.handleImportInsomnia },
          ],
        },
        {
          kind: 'submenu',
          label: 'Export',
          items: [
            {
              kind: 'action',
              label: 'Save Response Body…',
              disabled: !tabs.activeTab.response,
              onClick: () => {
                const body = tabs.activeTab.response?.body ?? ''
                const ext = body.trimStart().startsWith('{') || body.trimStart().startsWith('[') ? '.json' : '.txt'
                saveContentToFile(body, `response${ext}`)
              },
            },
            { kind: 'separator' },
            { kind: 'action', label: 'Bundle (environments + requests + scenarios)…', onClick: handleExportBundle },
          ],
        },
        { kind: 'separator' },
        { kind: 'action', label: 'Exit', onClick: quitApp },
      ],
    },
    viewMenu,
    helpMenu,
  ] : [
    {
      label: 'File',
      items: [
        { kind: 'action', label: 'Open Workspace…', onClick: workspace.handleOpenWorkspace },
        { kind: 'action', label: 'New Workspace…',  onClick: workspace.handleNewWorkspace },
        recentWorkspacesSubmenu,
        { kind: 'separator' },
        { kind: 'action', label: 'Exit', onClick: quitApp },
      ],
    },
    viewMenu,
    helpMenu,
  ]
}
