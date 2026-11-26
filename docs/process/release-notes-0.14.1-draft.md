# Clear Signal 0.14.1 (draft; delete after publishing)

## Fixes a coverage bug in 0.14.0. If you use recorded coverage, use this version instead

**What 0.14.0 did wrong.** It could move older coverage to the **wrong spec** when **both** of these were true:
- a scenario names its spec with a `../` path, such as `../specs/api.yaml`; and
- another file also exists at that path **as read from the workspace folder**, so outside the workspace.

The older results then showed under that outside file, while new runs went to the right spec. The app could show the wrong spec's coverage for such a scenario. Nothing was deleted, but **coverage for the outside spec could look higher than it was, so a gate on it could pass when it should not.**

**Now** the folder of the scenario that wrote the path decides, and a path that leaves the workspace is never read from the workspace folder.

**⚠️ If 0.14.0 already moved your coverage, this release does not move it back:** the saved entry does not say which spec it was recorded for. You can tell by `clrsig coverage list` showing the full path of a file outside your workspace that your tests did not mean.
- **If you copied `.clrsig-coverage` before you upgraded:** put the copy back **before** you run 0.14.1 on it. Coverage recorded after the copy is lost.
- **If you did not:** `clrsig coverage reset <the full path shown> --workspace <folder>` removes the misplaced entry, **with its older results**. Then run your scenarios again.

## Coming from 0.13.5 or earlier

**Read the [0.14.0 notes](https://github.com/fuvito/api-workbench/releases/tag/v0.14.0) first:** several things that passed now stop, and recorded coverage moves to new names. 0.14.1 changes nothing else about them.

**Full Changelog**: https://github.com/fuvito/api-workbench/compare/v0.14.0...v0.14.1
