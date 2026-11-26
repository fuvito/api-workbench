import { Fragment, useEffect, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { getCheckTable, type CheckRow } from '@/lib/bridge'
import { cn } from '@/lib/utils'
import { inputCls } from '@/lib/styles'

const SEVERITY_CLS: Record<string, string> = {
  failure: 'text-red-400',
  warning: 'text-amber-400',
  note: 'text-[var(--color-muted-foreground)]',
}

const cellCls = 'px-3 py-1.5 border border-[var(--color-border)] text-left align-top'

/** The text a search looks in: everything the row shows, without Markdown's marks. */
function searchText(row: CheckRow): string {
  return [row.id, row.title, row.section, row.severity ?? '', ...row.cells.map(c => c.text)]
    .join('\n').replace(/[`*]/g, '').toLowerCase()
}

// Cells are short inline Markdown; a link to a place in the document has nowhere to go here.
function Inline({ text }: { text: string }) {
  return (
    <ReactMarkdown
      components={{
        p: ({ children }) => <span>{children}</span>,
        a: ({ children }) => <span>{children}</span>,
        code: ({ children }) => <code className="bg-[var(--color-muted)] rounded px-1 font-mono">{children}</code>,
      }}
    >
      {text}
    </ReactMarkdown>
  )
}

/** The catalog of checks as a table that can be searched. */
export function CheckTable() {
  const [rows, setRows] = useState<CheckRow[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [query, setQuery] = useState('')
  const [section, setSection] = useState('')
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    getCheckTable().then(setRows).catch(() => setFailed(true))
  }, [])

  const indexed = useMemo(() => (rows ?? []).map(row => ({ row, text: searchText(row) })), [rows])
  const sections = useMemo(() => [...new Set((rows ?? []).map(r => r.section))], [rows])
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  const shown = indexed
    .filter(({ row, text }) => (!section || row.section === section) && words.every(w => text.includes(w)))
    .map(({ row }) => row)

  if (failed) return <p className="text-sm text-[var(--color-muted-foreground)]">Failed to load the table of checks.</p>
  if (!rows) return <div className="text-sm text-[var(--color-muted-foreground)] py-8 text-center">Loading…</div>

  function toggle(id: string) {
    setOpen(prev => {
      const next = new Set(prev)
      if (!next.delete(id)) next.add(id)
      return next
    })
  }

  return (
    <div className="flex flex-col gap-2 text-xs text-[var(--color-foreground)]">
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="search"
          aria-label="Search the checks"
          placeholder="Search by ID or any word, for example CS-RESP-005 or enum"
          value={query}
          onChange={e => setQuery(e.target.value)}
          className={cn(inputCls, 'flex-1 min-w-[16rem] px-3 py-1.5')}
          autoFocus
        />
        <select aria-label="Section" value={section} onChange={e => setSection(e.target.value)} className={cn(inputCls, 'px-2 py-1.5 cursor-pointer')}>
          <option value="">All sections</option>
          {sections.map(s => <option key={s} value={s}>{s.replace(/`/g, '')}</option>)}
        </select>
        <span role="status" className="text-[var(--color-muted-foreground)]">{shown.length} of {rows.length}</span>
      </div>
      {shown.length === 0 ? (
        <p className="py-6 text-center text-[var(--color-muted-foreground)]">
          Nothing in the list matches. If it is something we should check, or fail to say we do not, please report it.
        </p>
      ) : (
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[var(--color-muted)]">
              {['ID', 'Title', 'Severity', 'Section'].map(h => <th key={h} className={cn(cellCls, 'font-semibold')}>{h}</th>)}
            </tr>
          </thead>
          <tbody>
            {shown.map(row => {
              const isOpen = open.has(row.id)
              return (
                <Fragment key={row.id}>
                  <tr className="cursor-pointer hover:bg-[var(--color-muted)]/50" onClick={() => toggle(row.id)}>
                    <td className={cn(cellCls, 'whitespace-nowrap font-mono')}>
                      <button type="button" aria-expanded={isOpen} className="flex items-center gap-1">
                        {isOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                        {row.id}
                      </button>
                    </td>
                    <td className={cellCls}><Inline text={row.title} /></td>
                    <td className={cn(cellCls, SEVERITY_CLS[row.severity ?? ''])}>{row.severity}</td>
                    <td className={cn(cellCls, 'text-[var(--color-muted-foreground)]')}><Inline text={row.section} /></td>
                  </tr>
                  {isOpen && (
                    <tr>
                      <td colSpan={4} className={cn(cellCls, 'bg-[var(--color-muted)]/30')}>
                        <dl className="flex flex-col gap-1.5 leading-relaxed">
                          {row.cells.map(c => (
                            <div key={c.label}>
                              <dt className="font-semibold"><Inline text={c.label} /></dt>
                              <dd><Inline text={c.text} /></dd>
                            </div>
                          ))}
                        </dl>
                      </td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
          </tbody>
        </table>
      )}
    </div>
  )
}
