<p align="center"><em>A simple gist editor for CLI.</em></p>
<p align="center">
  <img src="./docs/screenshot.png" width="500">
</p>

<p align="center">
    <a href="https://b4b4r07.mit-license.org">
        <img src="https://img.shields.io/github/license/b4b4r07/gist" alt="License"/>
    </a>
    <a href="https://github.com/b4b4r07/gist/releases">
        <img
            src="https://img.shields.io/github/v/release/b4b4r07/gist"
            alt="GitHub Releases"/>
    </a>
    <br />
    <a href="https://b4b4r07.github.io/gist/">
        <img
            src="https://img.shields.io/website?down_color=lightgrey&down_message=donw&up_color=green&up_message=up&url=https%3A%2F%2Fb4b4r07.github.io%2Fgist"
            alt="Website"
            />
    </a>
    <a href="https://github.com/b4b4r07/gist/actions/workflows/release.yaml">
        <img
            src="https://github.com/b4b4r07/gist/actions/workflows/release.yaml/badge.svg"
            alt="GitHub Releases"
            />
    </a>
    <a href="https://github.com/b4b4r07/gist/blob/master/go.mod">
        <img
            src="https://img.shields.io/github/go-mod/go-version/b4b4r07/gist"
            alt="Go version"
            />
    </a>
</p>

## Features

- Super fast, super interactive.
- Easy to view, edit, upload and delete.
- Edit as you like, then just save it. It would be uploaded to your Gist.
- Simple and intuitive CLI UX, no complex flags or lots subcommands.
- One binary, just grab from GitHub Releases.

## Installation

Download the binary from [GitHub Releases][release] and drop it in your `$PATH`.

- [Darwin / Mac](https://github.com/b4b4r07/gist/releases/latest)
- [Linux](https://github.com/b4b4r07/gist/releases/latest)

**For macOS / [Homebrew](https://brew.sh/) user**:

```bash
brew install b4b4r07/tap/gist
```

**Using [afx](https://github.com/b4b4r07/afx), package manager for CLI**:

```yaml
github:
- name: b4b4r07/gist
  description: A simple gist editor for CLI
  owner: b4b4r07
  repo: gist
  release:
    name: gist
    tag: v1.2.6 ## NEED UPDATE!
    asset:
      filename: '{{ .Release.Name }}_{{ .OS }}_{{ .Arch }}.tar.gz'
      replacements:
        darwin: darwin
        amd64: arm64
  command:
    link:
    - from: gist
      to: gist
    env:
      GIST_USER: b4b4r07 ## NEED UPDATE!
```

## Versus

There are many other implements as the gist client (called "gister") such as the following that works on command-line:

- <https://github.com/defunkt/gist>
- <https://github.com/jdowner/gist>
- ...

## License

MIT

## Author

b4b4r07

[release]: https://github.com/b4b4r07/gist/releases
[license]: https://b4b4r07.mit-license.org
