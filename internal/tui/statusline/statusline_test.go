package statusline

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEmpty(t *testing.T) {
	var l Line
	if k, s := l.Shown(t0, nil); k != None || s != "" {
		t.Errorf("got %v %q", k, s)
	}
	if !l.Due(t0).IsZero() {
		t.Errorf("due %v", l.Due(t0))
	}
}

func TestEventsGo(t *testing.T) {
	var l Line
	l.Say(t0, "done")
	if k, s := l.Shown(t0.Add(time.Second), nil); k != Info || s != "done" {
		t.Errorf("got %v %q", k, s)
	}
	if k, _ := l.Shown(t0.Add(InfoFor), nil); k != None {
		t.Errorf("still shown as %v", k)
	}
	if got := l.Due(t0); !got.Equal(t0.Add(InfoFor)) {
		t.Errorf("due %v", got)
	}
}

func TestActivityShowsWhenNothingElseDoes(t *testing.T) {
	var l Line
	if k, s := l.Shown(t0, []string{"reading", "saving"}); k != Activity || s != "reading · saving" {
		t.Errorf("got %v %q", k, s)
	}
	l.Say(t0, "done")
	if k, _ := l.Shown(t0, []string{"reading"}); k != Info {
		t.Errorf("got %v", k)
	}
}

func TestAFailureOutlastsInformation(t *testing.T) {
	var l Line
	l.Fail(t0, "failed")
	l.Say(t0.Add(time.Second), "done")
	if k, s := l.Shown(t0.Add(2*time.Second), nil); k != Failure || s != "failed" {
		t.Errorf("got %v %q", k, s)
	}
}

func TestRefusalGoesAtTheNextKey(t *testing.T) {
	var l Line
	l.Refuse(t0, "no")
	l.KeyPressed()
	if k, _ := l.Shown(t0, nil); k != None {
		t.Errorf("got %v", k)
	}
	l.Say(t0, "yes")
	l.KeyPressed()
	if k, _ := l.Shown(t0, nil); k != Info {
		t.Errorf("information went at a key: %v", k)
	}
}
