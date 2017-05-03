<a href="https://gist.github.com"><img src="https://raw.githubusercontent.com/b4b4r07/i/master/gist/logo.png" width="200"></a>

> A simple gist editor for CLI
> 
> <a href="https://github.com/b4b4r07/gist"><img src="https://octodex.github.com/images/megacat-2.png" width="200"></a>

## Pros

- Simple and intuitive
    - Just select and edit gist you want
        - Can use any editors what you want
        - Work with [peco](https://github.com/peco/peco) and [fzf](https://github.com/junegunn/fzf)
    - Automatically synchronized after editing
- Customizable
    - A few options and small TOML
- Easy to install
    - Go! single binary

***DEMO***

<a href="https://github.com/b4b4r07/gist"><img src="https://raw.githubusercontent.com/b4b4r07/i/master/gist/demo.gif" width="500"></a>

## Usage

Currently gist supports the following commands:

```console
$ gist help
gist - A simple gist editor for CLI

Usage:
  gist [flags]
  gist [command]

Available Commands:
  config      Config the setting file
  delete      Delete gist files
  edit        Edit the gist file and sync after
  new         Create a new gist
  open        Open user's gist

Flags:
  -v, --version   show the version and exit

Use "gist [command] --help" for more information about a command.
```

### Configurations

Well-specified options and user-specific settings can be described in a toml file. It can be changed with the `gist set` command.

```toml
[Core]
  Editor = "vim"
  selectcmd = "fzf-tmux --multi:fzf:peco:percol"
  tomlfile = "/Users/b4b4r07/.config/gist/config.toml"
  user = "b4b4r07"

[Gist]
  token = "your_github_token"
  dir = "/Users/b4b4r07/.config/gist/files"

[Flag]
  open_url = false
  private = false
  verbose = true
  show_spinner = true
```

This behavior was heavily inspired by [mattn/memo](https://github.com/mattn/memo), thanks!

## Installation

```console
$ go get github.com/b4b4r07/gist
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
