# Changelog

## [v2.1.0](https://github.com/babarot/gist/compare/v2.0.0...v2.1.0) - 2026-11-07
### New Features
- Copy the file shown with Y by @babarot in https://github.com/babarot/gist/pull/45
- Show the size of a gist in the list, and filter by it by @babarot in https://github.com/babarot/gist/pull/48
- Filter by owner, by when a gist changed and by its comments by @babarot in https://github.com/babarot/gist/pull/49
### Bug fixes
- Bring back --version by @babarot in https://github.com/babarot/gist/pull/47
- Stop losing edits to names that differ in case and to failed deletes by @babarot in https://github.com/babarot/gist/pull/50
- Lock gists on Windows, and run the tests there by @babarot in https://github.com/babarot/gist/pull/53
### Improvements
- Show the release notes in place of the key list by @babarot in https://github.com/babarot/gist/pull/43
- Fade the tabs of the preview into its frame by @babarot in https://github.com/babarot/gist/pull/46
### Refactorings
- Test what HTML in Markdown becomes, and steady the long line test by @babarot in https://github.com/babarot/gist/pull/51
- Remove what nothing uses, and test where fuzzy matching moves a hit by @babarot in https://github.com/babarot/gist/pull/54

## [v2.0.0](https://github.com/babarot/gist/compare/v1.2.8...v2.0.0) - 2026-11-07
### Breaking Changes
- Replace the v1 code with the foundation of v2 by @babarot in https://github.com/babarot/gist/pull/25
- Rewrite gist as a TUI and a scriptable CLI by @babarot in https://github.com/babarot/gist/pull/42
### New Features
- Add the GitHub layer, the list cache and gist auth by @babarot in https://github.com/babarot/gist/pull/26
- Add the CLI and the shared edit logic by @babarot in https://github.com/babarot/gist/pull/27
- Add the skeleton of the TUI by @babarot in https://github.com/babarot/gist/pull/28
- Draw the preview of a gist by @babarot in https://github.com/babarot/gist/pull/29
- Edit gists from the TUI and gist edit by @babarot in https://github.com/babarot/gist/pull/30
- Add the filter and the form of a new gist by @babarot in https://github.com/babarot/gist/pull/31
- Complete the filter with tab and add height keys, log and config check by @babarot in https://github.com/babarot/gist/pull/36
- Show the version in the key list, with the release notes by @babarot in https://github.com/babarot/gist/pull/37
### Bug fixes
- Count the cursor of a text box in cells by @babarot in https://github.com/babarot/gist/pull/35
### Improvements
- Page through the list and keep the file shown in sight by @babarot in https://github.com/babarot/gist/pull/32
- Show a scrollbar on the preview by @babarot in https://github.com/babarot/gist/pull/33
- Give the owner of a starred gist a column of its own by @babarot in https://github.com/babarot/gist/pull/34
- Wrap the lines of a diff and mark what changed within them by @babarot in https://github.com/babarot/gist/pull/38
- Dim what is under a box by @babarot in https://github.com/babarot/gist/pull/39
- Add a demo, and rewrite the README and the page for v2 by @babarot in https://github.com/babarot/gist/pull/40
- Drop new -p, and name the update check after gist by @babarot in https://github.com/babarot/gist/pull/41
### Others
- Sync shared files from github-config by @babarot in https://github.com/babarot/gist/pull/21

## [v1.2.8](https://github.com/babarot/gist/compare/v1.2.7...v1.2.8) - 2026-10-09
### Bug fixes
- Show the version the release was built with by @babarot in https://github.com/babarot/gist/pull/19

## [v1.2.7](https://github.com/babarot/gist/compare/v1.2.6...v1.2.7) - 2026-10-05
### Others
- chore: sync shared files from github-config by @babarot in https://github.com/babarot/gist/pull/13
