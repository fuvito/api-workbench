export interface Release {
  version: string
  date: string
  items: string[]
}

const HEADING = /^## (\d+\.\d+\.\d+) — (\d{4}-\d{2}-\d{2})\s*$/

/** Reads help/changelog.md: a "## version — date" heading per release and a "- " line per change. */
export function parseChangelog(markdown: string): Release[] {
  const releases: Release[] = []
  for (const line of markdown.split(/\r?\n/)) {
    const heading = HEADING.exec(line)
    if (heading) releases.push({ version: heading[1], date: heading[2], items: [] })
    else if (line.startsWith('- ')) releases.at(-1)?.items.push(line.slice(2).trim())
  }
  return releases
}
