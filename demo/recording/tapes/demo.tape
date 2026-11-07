# The demo in the README: read the gists, filter them, edit one and check the diff
# before it is pushed.
#
# Run with: mise run demo
# gist runs for real against a made-up GitHub (demo/internal/recording).

Output demo.mp4

# The colors of gist's default theme
Set Theme { "name": "gist", "black": "#1e1e2e", "red": "#f5a3b5", "green": "#b5f0b0", "yellow": "#ffe5a0", "blue": "#96bfff", "magenta": "#d4b0ff", "cyan": "#94e2d5", "white": "#d0d6e8", "brightBlack": "#706f87", "brightRed": "#f5a3b5", "brightGreen": "#b5f0b0", "brightYellow": "#ffe5a0", "brightBlue": "#96bfff", "brightMagenta": "#d4b0ff", "brightCyan": "#94e2d5", "brightWhite": "#e0e4f0", "background": "#1e1e2e", "foreground": "#e0e4f0", "selection": "#3b3b52", "cursor": "#e0e4f0" }

Set Shell "bash"
Set FontSize 16
Set Width 1400
Set Height 760
Set Padding 20
Set TypingSpeed 60ms
Set Framerate 30

Hide
Type "export COLORTERM=truecolor PS1='$ ' EDITOR=/data/edit.sh; gist() { /data/.gist-demo; }; clear"
Enter
Sleep 500ms
Show

Sleep 500ms
Type "gist"
Sleep 400ms
Enter
Sleep 2.5s

# Look over the gists, and the files of one
Type "j"
Sleep 1s
Type "c"
Sleep 2s
Type "c"
Sleep 500ms
Type "j"
Sleep 1.2s
Type "l"
Sleep 900ms
Type "l"
Sleep 1.2s
Type "J"
Sleep 300ms
Type "J"
Sleep 1s

# Narrow the list down, with tab filling in the key and its values
Type "/"
Sleep 1s
Type "vi"
Sleep 800ms
Tab
Sleep 900ms
Tab
Sleep 1.2s
Tab
Sleep 1.5s
Enter
Sleep 900ms
Escape
Sleep 800ms

# The gists starred, with whose they are
Type "2"
Sleep 1.2s
Type "j"
Sleep 900ms
Type "1"
Sleep 800ms

# Edit one, and read the diff before it goes
Type "g"
Sleep 900ms
Enter
Sleep 2.5s
Type "y"
Sleep 2.5s

# The keys, and what is new
Type "?"
Sleep 2s
Type "w"
Sleep 2.5s
Escape
Sleep 500ms
Escape
Sleep 500ms
Type "q"
Sleep 1s
