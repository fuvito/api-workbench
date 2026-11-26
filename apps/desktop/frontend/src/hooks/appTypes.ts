// The app's main hooks, as the smaller hooks that work with them take them.
import type { useDemo } from '@/hooks/useDemo'
import type { useSettings } from '@/hooks/useSettings'
import type { useTabsState } from '@/hooks/useTabsState'
import type { useWorkspace } from '@/hooks/useWorkspace'

export type Tabs = ReturnType<typeof useTabsState>
export type Workspace = ReturnType<typeof useWorkspace>
export type Settings = ReturnType<typeof useSettings>
export type Demo = ReturnType<typeof useDemo>
