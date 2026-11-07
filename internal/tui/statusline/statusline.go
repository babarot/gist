// Package statusline holds what the status line shows: what just happened, for
// a moment, and what is going on.
package statusline

import (
	"strings"
	"time"
)

// Kind is what the line shows.
type Kind int

const (
	// None is an empty line.
	None Kind = iota
	// Info is a success or other information.
	Info
	// Refusal says why the key pressed cannot act here.
	Refusal
	// Failure says that something went wrong.
	Failure
	// Activity is the work in progress.
	Activity
)

// How long each kind of event stays: a failure longer, as it may need reading
// twice.
const (
	InfoFor    = 4 * time.Second
	RefusalFor = 4 * time.Second
	FailureFor = 10 * time.Second
)

type event struct {
	kind Kind
	text string
	at   time.Time
}

func (e event) end() time.Time {
	switch e.kind {
	case Refusal:
		return e.at.Add(RefusalFor)
	case Failure:
		return e.at.Add(FailureFor)
	}
	return e.at.Add(InfoFor)
}

func (e event) live(now time.Time) bool { return e.text != "" && now.Before(e.end()) }

// Line is the state of the status line. The zero Line is empty.
type Line struct {
	last   event
	failed event
}

// Say tells of something that happened.
func (l *Line) Say(now time.Time, text string) { l.tell(Info, now, text) }

// Refuse tells why the key pressed cannot act here. It goes at the next key.
func (l *Line) Refuse(now time.Time, text string) { l.tell(Refusal, now, text) }

// Fail tells of something that went wrong.
func (l *Line) Fail(now time.Time, text string) { l.tell(Failure, now, text) }

func (l *Line) tell(k Kind, now time.Time, text string) {
	e := event{kind: k, text: text, at: now}
	l.last = e
	if k == Failure {
		l.failed = e
	}
}

// KeyPressed puts away a refusal, which answers the key before.
func (l *Line) KeyPressed() {
	if l.last.kind == Refusal {
		l.last = event{}
	}
}

// Shown is what the line shows at now, given the work in progress: the newest
// event still in its time, information giving way to a failure still in its
// time, then the work in progress.
func (l *Line) Shown(now time.Time, activities []string) (Kind, string) {
	if l.last.live(now) && l.last.kind != Info {
		return l.last.kind, l.last.text
	}
	if l.failed.live(now) {
		return Failure, l.failed.text
	}
	if l.last.live(now) {
		return Info, l.last.text
	}
	if len(activities) > 0 {
		return Activity, strings.Join(activities, " · ")
	}
	return None, ""
}

// Due is the next moment after now at which what the line shows may change by
// time alone, zero when none is coming.
func (l *Line) Due(now time.Time) time.Time {
	var due time.Time
	for _, t := range []time.Time{l.last.end(), l.failed.end()} {
		if t.After(now) && (due.IsZero() || t.Before(due)) {
			due = t
		}
	}
	return due
}
