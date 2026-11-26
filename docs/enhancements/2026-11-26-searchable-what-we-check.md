# A searchable "What We Check" table (B-097)

**Status:** Done, not yet released. Accepted by the owner, 2026-11-26 ("go" on the pre-release batch).

## Summary

The catalog of what Clear Signal checks is shown as one long document. Someone with an ID from a report, or asking "do you check X?", has to scroll. The Help panel's **What We Check** tab gets a table of every row, with search and a section filter. The document stays one click away.

## Goals / non-goals

**Goals:** find a row by ID or by any word in it; see only one section, such as the known gaps; see each finding's severity; read all of a row without leaving the table.

**Non-goals:** a new rail page, a CLI command, editing, links from reports into the table, the web site.

## Requirement review

Backlog B-097, "from the check data". `packages/core/checks` holds only the findings a run can report (ID, title, severity, area). The known gaps and the described behaviours, which people most need to search, are only in the catalog document. So the table is **the document's rows, joined with the check data for severity**. A second list of all rows in Go was rejected: two copies of long text would drift. No new dependency, no format change.

## Design

- `checks.ParseCatalog(markdown)` returns one row per table row of the catalog: ID, title, section heading, the other columns as label and text in their order, and the severity from `checks.All()` (empty for a row that is not a finding). The caller passes the document, so the core does not depend on the desktop app.
- `HelpService.GetCheckTable()` parses the embedded document. The front end only filters and shows.
- **Table:** ID, title, severity and section. A row opens to show its other columns, as inline Markdown. The search matches every word typed, in any column, in any case. A count says how many rows are shown.
- The tab opens on the table; **Full document** shows the text as before. Copy and Download still give the document.
- **Security:** only the app's own embedded text is shown.

## Phasing and acceptance criteria

One phase. Accepted when every row of the catalog is in the table with all its cells, and searching an ID leaves that row.

## Testing

A Go test against the real document (every row present, cell counts, escaped pipes, severities), front-end tests for search, filter and opening a row, and the existing browser tests.

## Documentation impact

User guide (Help), the catalog's introduction, engineering changelog, backlog.

## Risks

A table in the catalog with another layout (no ID first column) would be left out; the Go test counts the rows.

## Open decisions

None.

## Decision log

- 2026-11-26: accepted with the batch (owner "go").
- 2026-11-26: the rows come from the document, joined with the check data (see Requirement review).
