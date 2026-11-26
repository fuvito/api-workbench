import { useEffect, useRef, useState } from 'react'

/** The widths of the sidebar and the response panel: dragged, and remembered between sessions. */
export function usePanelSizes() {
  const [sidebarWidth, setSidebarWidth] = useState(() => {
    const v = parseInt(localStorage.getItem('ui.sidebarWidth') ?? '')
    return isNaN(v) ? 224 : Math.max(160, Math.min(480, v))
  })
  const [responsePct, setResponsePct] = useState(() => {
    const v = parseFloat(localStorage.getItem('ui.responsePct') ?? '')
    return isNaN(v) || v === 0 ? 50 : Math.min(80, Math.max(10, v))
  })
  const draggingRef = useRef<'sidebar' | 'response' | null>(null)
  const sidebarWidthRef = useRef(sidebarWidth)
  const responsePctRef = useRef(responsePct)
  useEffect(() => { sidebarWidthRef.current = sidebarWidth }, [sidebarWidth])
  useEffect(() => { responsePctRef.current = responsePct }, [responsePct])

  // Resize
  useEffect(() => {
    function onMouseMove(e: MouseEvent) {
      if (draggingRef.current === 'sidebar') {
        setSidebarWidth(prev => Math.max(160, Math.min(480, prev + e.movementX)))
      } else if (draggingRef.current === 'response') {
        setResponsePct(prev => {
          const availableWidth = window.innerWidth - sidebarWidthRef.current
          const deltaPct = -(e.movementX / availableWidth) * 100
          const next = Math.min(80, Math.max(10, prev + deltaPct))
          responsePctRef.current = next
          return next
        })
      }
    }
    function onMouseUp() {
      if (draggingRef.current === 'sidebar') {
        localStorage.setItem('ui.sidebarWidth', String(sidebarWidthRef.current))
      } else if (draggingRef.current === 'response') {
        localStorage.setItem('ui.responsePct', String(Math.round(responsePctRef.current)))
      }
      draggingRef.current = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }
  }, [])

  function startDrag(panel: 'sidebar' | 'response', e: React.MouseEvent) {
    e.preventDefault()
    draggingRef.current = panel
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  return { sidebarWidth, responsePct, startDrag }
}
