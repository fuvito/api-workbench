# Keep `x-` keys when the app saves a file (B-153)

**Status:** Done, not yet released. Accepted by the owner, 2026-11-26 ("go" on the pre-release batch).

## Summary

Request, scenario and environment files may hold the author's own `x-` keys (B-121). Every save that rebuilds a file from the editor or the domain model drops them, against `CLAUDE.md` section 8 ("preserve unknown supported extension fields where possible"). The keys travel with the item they belong to, through the Go model and the editors, and are written back.

## Goals / non-goals

**Goals:** an `x-` key survives opening, editing and saving a request, scenario or environment, at the top level and on a step, step request, header, query parameter, body, auth, retry, assertion, extraction and variable. It stays with **its own item** when items are renamed, reordered, added or removed.

**Non-goals:**
- Comments, key order, quoting and anchors. A rebuilt file does not keep them (unchanged). The raw YAML editors save the text as typed.
- Bundles. A bundle stays strict: importing one that has `x-` keys is refused as before, and **export leaves them out**. A bundle is made to be shared, and nothing redacts free-form `x-` content.
- Keys inherited through a YAML merge (`<<:`).
- Editing `x-` keys in the forms.

## Requirement review

Required by the file format contract. No new dependency, no schema version change, no change to what any command does with a file. Several packages and both editors are touched, hence this document. A smaller alternative, matching old and new items by position or name when saving, was rejected: a reordered or renamed step would get another step's keys.

## Design

- **Model:** `domain.Extensions` is a string: one line per key, `"x-owner": <value as compact JSON>`, in file order. A string keeps the domain structs comparable, and the front end can carry it without reading it, so large numbers are not rounded by JavaScript.
- **Reading** (`serialization`): after the file is decoded and its keys checked, the document tree is walked beside the file struct and each mapping's `x-` pairs are put on that struct. The decoder's rules do not change.
- **Writing:** the struct is encoded to a document tree, each struct's pairs are added at the end of its mapping, and the tree is written as YAML or JSON. Without extensions the output is what it was.
- **Desktop:** every input/output type that mirrors one of these structs has an `extensions` string. The editors keep it on the same object as the row it belongs to, and `scenario-yaml.ts` and the environment form write the lines back.
- **Copies** (duplicate tab, request or environment) keep the keys.
- **Removed things lose their keys:** a body set to "none", a changed auth type, a removed retry policy.
- **Security:** the keys are the user's own local file content. They are not added to reports, history or bundles.

## Phasing and acceptance criteria

One phase. Accepted when, for each file type, parse then write keeps every `x-` key at every level with its value's type, and when saving from each editor after a reorder keeps each key on its item.

## Testing

Go round trips at every level (YAML and JSON), bundle export without them, the desktop save paths, and front-end tests for the scenario writer, the request tab and the environment form.

## Documentation impact

`format-spec.md` ("Unknown keys"), `what-we-check.md` (CS-RUN-024), the engineering changelog and the backlog.

## Risks

- A new field added to an editor type and not copied loses keys again. The tests cover the existing paths only.
- A YAML date or timestamp in an `x-` value is written back quoted (a string).

## Open decisions

None.

## Decision log

- 2026-11-26: accepted with the batch; keys follow the item, not its position (owner "go").
- 2026-11-26: bundle export leaves `x-` keys out (the safer choice for a shared file; follows the recommendation rule in the "go").
