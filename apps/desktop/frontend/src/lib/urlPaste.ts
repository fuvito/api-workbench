import { parseCurl } from '@/lib/curlImport'
import type { TabState } from '@/lib/tabState'
import type { KVPair } from '@/types'

/** What pasting into the URL field does: a cURL command fills the request, a query string its parameters. */
export function applyUrlPaste(
  raw: string,
  activeTab: TabState,
  updateTab: (id: string, patch: Partial<TabState>) => void,
): void {
  const activeTabId = activeTab.id
  try {
    const curlResult = parseCurl(raw)
    if (curlResult) {
      updateTab(activeTabId, {
        method: curlResult.method,
        url: curlResult.url,
        queryParams: curlResult.queryParams,
        headers: curlResult.headers,
        body: curlResult.body,
        auth: curlResult.auth,
      })
      return
    }
    const qIdx = raw.indexOf('?')
    if (qIdx <= 0) return  // no '?' or starts with '?' — don't clobber existing URL
    const baseUrl = raw.slice(0, qIdx)
    const search = raw.slice(qIdx + 1)
    const parsed = new URLSearchParams(search)
    const extracted: KVPair[] = []
    parsed.forEach((value, name) => {
      extracted.push({ id: crypto.randomUUID(), name, value, enabled: true })
    })
    updateTab(activeTabId, { url: baseUrl })
    if (extracted.length > 0) {
      const names = new Set(extracted.map(p => p.name.toLowerCase()))
      const kept = activeTab.queryParams.filter(p => !names.has(p.name.toLowerCase()))
      updateTab(activeTabId, { queryParams: [...kept, ...extracted] })
    }
  } catch {
    updateTab(activeTabId, { url: raw, saveError: 'Could not extract params from pasted URL' })
    setTimeout(() => updateTab(activeTabId, { saveError: null }), 3000)
  }
}
