## Read before upgrading

Some runs that passed can now fail. Details: user guide, Troubleshooting, **Upgrading to 0.15.0**.

- An unknown key in a request, scenario or environment file is refused, with the spelling probably meant. Keys of your own start with `x-`.
- `format: uri` and `format: hostname` are now checked.
- With `conform --negative --fail-on warning`, a refusal that may be for another reason, and a rate-limited probe, fail the run.

## Fixed

- **Saving from the visual scenario editor lost data:** an assertion's `name`, a step request's `title` and `timeout` (so the saved scenario could run differently), and a list or object under `expected` was written as text. If you saved scenarios with 0.14.1 or earlier, compare them with your version control.
- Saving in the app keeps your `x-` keys, each on its own item.
- Negative probes: a refusal with a status that does not fit the fault now warns, and a `429` is reported as not checked instead of passed.

## New

- `--max-response-time-for` also takes a tag (`tag:search=800`) or one operation (`"GET /export=0"`).
- GitHub pull request annotations for `diff` and `bundle run` (`--format github`).
- `bundle run --workspace` records coverage, and the bundle import dialog can copy its command.
- **What We Check** opens as a searchable table.
- The About dialog shows the release notes.
- The Demo and Conformance Server workspaces include a `clrsig.yaml`.
