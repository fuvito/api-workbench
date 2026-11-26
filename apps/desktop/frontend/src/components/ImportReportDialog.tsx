import type { CollectionImportResult } from '@/lib/bridge'

/** What a Postman or Insomnia import could not convert. */
export function ImportReportDialog({ report, onClose }: { report: CollectionImportResult; onClose: () => void }) {
  const warnings = report.warnings ?? []
  const imported = report.imported?.length ?? 0
  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center bg-black/60">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="import-report-title"
        className="bg-[var(--color-card)] border border-[var(--color-border)] rounded-lg p-5 w-[34rem] max-w-[90vw] max-h-[80vh] shadow-2xl flex flex-col gap-3"
        onKeyDown={e => { if (e.key === 'Escape') onClose() }}
      >
        <h3 id="import-report-title" className="text-sm font-semibold text-[var(--color-foreground)]">
          Imported {imported} {imported === 1 ? 'request' : 'requests'} from {report.collectionName}
        </h3>
        {report.skipped > 0 && (
          <p className="text-xs text-red-400">
            {report.skipped} {report.skipped === 1 ? 'request' : 'requests'} could not be written.
          </p>
        )}
        {warnings.length > 0 && (
          <>
            <p className="text-xs text-[var(--color-muted-foreground)]">
              Check these before you rely on the imported requests. This list is not saved, so copy it if you need it later.
            </p>
            <ul className="overflow-auto text-xs text-[var(--color-foreground)] border border-[var(--color-border)] rounded p-2 flex flex-col gap-1 select-text">
              {warnings.map((w, i) => <li key={i}>{w}</li>)}
            </ul>
          </>
        )}
        <div className="flex justify-end">
          <button
            type="button"
            autoFocus
            onClick={onClose}
            className="px-3 py-1 text-xs rounded bg-[var(--color-muted)] text-[var(--color-foreground)] hover:bg-[var(--color-border)] transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
