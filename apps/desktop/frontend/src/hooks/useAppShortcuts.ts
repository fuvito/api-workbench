import { useEffect, type Dispatch, type RefObject, type SetStateAction } from 'react'
import type { Tabs } from '@/hooks/appTypes'

type Params = {
  tabs: Tabs
  handleSendRef: RefObject<(() => void) | null>
  workspacePathRef: RefObject<string>
  setSidebarOpen: Dispatch<SetStateAction<boolean>>
  setShowQuickOpen: Dispatch<SetStateAction<boolean>>
}

export function useAppShortcuts({ tabs, handleSendRef, workspacePathRef, setSidebarOpen, setShowQuickOpen }: Params) {
  // Keyboard shortcuts
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === 'S' && e.shiftKey) {
        e.preventDefault()
        tabs.handleSaveAll()
      } else if ((e.ctrlKey || e.metaKey) && e.key === 's' && !e.shiftKey) {
        e.preventDefault()
        tabs.saveTabByIdRef.current(tabs.tabsStateRef.current.activeTabId)
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault()
        tabs.handleNewTab()
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'w') {
        e.preventDefault()
        tabs.handleCloseTab(tabs.tabsStateRef.current.activeTabId)
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        const el = document.activeElement
        if (el instanceof HTMLTextAreaElement) return
        e.preventDefault()
        handleSendRef.current?.()
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'b') {
        e.preventDefault()
        setSidebarOpen(o => {
          const next = !o
          localStorage.setItem('ui.sidebarOpen', String(next))
          return next
        })
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'p') {
        if (!workspacePathRef.current) return
        e.preventDefault()
        setShowQuickOpen(o => !o)
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'Tab') {
        e.preventDefault()
        const { tabs: tList, activeTabId } = tabs.tabsStateRef.current
        if (tList.length < 2) return
        const idx = tList.findIndex(t => t.id === activeTabId)
        const next = e.shiftKey
          ? (idx - 1 + tList.length) % tList.length
          : (idx + 1) % tList.length
        tabs.switchTo(tList[next].id)
      }
      const digit = parseInt(e.key)
      if ((e.ctrlKey || e.metaKey) && digit >= 1 && digit <= 9) {
        const { tabs: tList } = tabs.tabsStateRef.current
        const target = tList[digit - 1]
        if (target) { e.preventDefault(); tabs.switchTo(target.id) }
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
}
