// Package near finds, among a few names, the one closest to a name that
// was mistyped, so a message can ask "did you mean ...?".
package near

// Closest is the name among names nearest to s, when it is near enough to
// be what was meant: a few letters added, dropped, changed or swapped.
// The first of the nearest wins.
func Closest(s string, names []string) (string, bool) {
	best, bestD := "", -1
	for _, n := range names {
		if n == s {
			continue
		}
		d := distance(s, n)
		if bestD < 0 || d < bestD {
			best, bestD = n, d
		}
	}
	// One mistake in a short name, about one in three in a longer one.
	limit := max(1, len([]rune(s))/3)
	if bestD < 0 || bestD > limit {
		return "", false
	}
	return best, true
}

// distance is the edit distance between a and b, with swapping two
// letters next to each other counted as one edit.
func distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	d := make([][]int, len(x)+1)
	for i := range d {
		d[i] = make([]int, len(y)+1)
		d[i][0] = i
	}
	for j := range y {
		d[0][j+1] = j + 1
	}
	for i := 1; i <= len(x); i++ {
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && x[i-1] == y[j-2] && x[i-2] == y[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(x)][len(y)]
}
