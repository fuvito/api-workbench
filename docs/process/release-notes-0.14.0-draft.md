# Clear Signal 0.14.0 (draft; delete after publishing)

## ⚠️ Read before you upgrade: some things that passed now stop

No file format changed and there is nothing to convert. **Check your pipelines for these:**

| If | Now | What to do |
|---|---|---|
| `coverage quality --fail-on-gaps` runs before anything was recorded (a misspelled name included) | **It fails.** Before, it passed with nothing checked | Record coverage in the same job, or keep the workspace folder between jobs |
| A `coverage` command names a workspace folder that does not exist | Exit `2` | Correct the name, or create the folder |
| A request, scenario or environment file has a `schemaVersion` other than `"1"` | **Refused.** A file with no version is still read | Correct it to `"1"` |
| `clrsig request`, the app's request editor or **Run folder** meet a `{{name}}` nothing defines | **Not sent.** Before, the braces went to the server | Give the value. Text that must go out with `{{ }}` can no longer be sent this way |
| A secret still holds the `CHANGE_ME` that `bundle export` wrote | It has **no value**, in every command and the app; a request that uses it is not sent. Before, `CHANGE_ME` was sent as the secret | Put the real value in the environment, or use `--var` |
| A scenario has an HTTP method that does not exist; `validate` cannot read a file | `validate` reports it; an unreadable file is exit `2` | Correct the file or path |

**One check is more lenient, as its help always said:** `coverage quality` with `csv`, `markdown` or `html` no longer exits 1 on gaps without `--fail-on-gaps`. **If a job relied on that, add the flag.**

## ⚠️ Recorded coverage moves to new names

Coverage is now kept per **spec file** (however its path was written) or per exact **web address**. Before, specs with similar names could share one entry. Spec files are shown by their path from the workspace root, or in full when outside it.

- An older entry is renamed the first time it is read, **when its spec can be told for certain**. Otherwise it is kept under its old name.
- **Do not go back to 0.13.5 or earlier on the same workspace:** it does not find renamed entries and starts new ones. If your CI caches the workspace, use one version for that cache.
- To be able to go back, copy the workspace's `.clrsig-coverage` folder before upgrading.

## Fixed

- `coverage list` and `quality` ignored `--output` for `text` and `json`: nothing was written.
- `coverage reset` said it had reset when there was nothing to reset.
- `clrsig import` now warns for each request that had a Postman test or pre-request script; **the imported requests do not check what those scripts checked.** The app now lists import warnings too.
- The telemetry notice (off by default) now names everything sent, including a random identifier of the installation. It no longer says "anonymous".
- The app's coverage cards say whether a spec is a file or a web address. A file and an address of the same spec are still two cards.

## The detailed command-line guide is complete

New: [pages by task](https://github.com/fuvito/api-workbench/blob/main/docs/guides/cli/README.md), [exit codes and report formats](https://github.com/fuvito/api-workbench/blob/main/docs/guides/cli/reference.md) and [troubleshooting by message](https://github.com/fuvito/api-workbench/blob/main/docs/guides/cli/troubleshooting.md).

**Full Changelog**: https://github.com/fuvito/api-workbench/compare/v0.13.5...v0.14.0
