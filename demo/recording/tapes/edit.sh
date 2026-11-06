#!/bin/sh
# The editor of the recording: it changes the end of the long line of minutes.md,
# past where a row of the diff ends.
sed -i 's/whether a cache is worth adding\.$/whether a cache is worth adding. Ana owns the numbers./' "$1"
