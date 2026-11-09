package tui

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/fakegist"
)

// several is a world with n gists of the mine tab, newest last made.
func several(t *testing.T, n int) (*world, []*fakegist.Gist) {
	t.Helper()
	var gs []*fakegist.Gist
	w := newWorld(t, func(s *fakegist.Server) {
		for i := range n {
			gs = append(gs, s.Add(fmt.Sprintf("gist %d", i), false, map[string]string{fmt.Sprintf("f%d.txt", i): "x\n"}))
		}
	})
	return w, gs
}

func selectedIDs(m *Model) []string {
	ids := m.selectedIDs()
	slices.Sort(ids)
	return ids
}

func deletes(s *fakegist.Server) int {
	n := 0
	for _, r := range s.Requests() {
		if strings.HasPrefix(r, "DELETE ") {
			n++
		}
	}
	return n
}

func TestSpaceSelectsAndGoesDown(t *testing.T) {
	w, _ := several(t, 3)
	first, _ := w.m.current()
	press(t, w.m, " ")
	if !w.m.selected[first.ID] || w.m.tab().cur != 1 {
		t.Fatalf("selected %v, cursor at %d", w.m.selected, w.m.tab().cur)
	}
	press(t, w.m, " ")
	w.wants("2 selected", "✓")
	press(t, w.m, "k", " ")
	if len(w.m.selected) != 1 || !w.m.selected[first.ID] {
		t.Errorf("selected %v", w.m.selected)
	}
}

func TestSelectShownTakesWhatTheFilterLeaves(t *testing.T) {
	w, gs := several(t, 3)
	press(t, w.m, "/")
	typeText(t, w.m, "gist 1")
	enter(t, w.m)
	press(t, w.m, "V")
	if got := selectedIDs(w.m); !slices.Equal(got, []string{gs[1].ID}) {
		t.Errorf("selected %v", got)
	}
}

func TestClearAndRestoreTheSelection(t *testing.T) {
	w, _ := several(t, 3)
	press(t, w.m, " ", " ")
	was := selectedIDs(w.m)
	press(t, w.m, "u")
	if len(w.m.selected) != 0 {
		t.Fatalf("selected %v", w.m.selected)
	}
	w.wants("u restore selection")
	press(t, w.m, "u")
	if got := selectedIDs(w.m); !slices.Equal(got, was) {
		t.Errorf("selected %v, was %v", got, was)
	}
	w.wants("selected again: 2 gists")
	fresh, _ := several(t, 1)
	press(t, fresh.m, "u")
	fresh.wants("no selection to bring back")
}

func TestEscClearsTheSelectionBeforeTheFilter(t *testing.T) {
	w, _ := several(t, 3)
	press(t, w.m, "/")
	typeText(t, w.m, "gist")
	enter(t, w.m)
	press(t, w.m, " ", "esc")
	if len(w.m.selected) != 0 || w.m.tab().query != "gist" {
		t.Fatalf("selected %v, filter %q", w.m.selected, w.m.tab().query)
	}
	press(t, w.m, "esc")
	if w.m.tab().query != "" {
		t.Errorf("filter %q", w.m.tab().query)
	}
}

func TestStarredGistsAreNotSelected(t *testing.T) {
	w := newWorld(t, func(s *fakegist.Server) {
		s.Add("mine", false, map[string]string{"a.txt": "a\n"})
		s.Star("kanata", "theirs", map[string]string{"b.txt": "b\n"})
	})
	press(t, w.m, "2")
	for _, k := range []string{" ", "V"} {
		press(t, w.m, k)
		w.wants("read only: kanata's gist")
	}
	if len(w.m.selected) != 0 {
		t.Errorf("selected %v", w.m.selected)
	}
}

func TestKeysForOneGistRefuseWithASelection(t *testing.T) {
	w, _ := several(t, 2)
	press(t, w.m, " ")
	for _, k := range []string{"e", "E", "r", "Y"} {
		press(t, w.m, k)
		w.wants("one gist at a time · u clears the selection")
		if w.m.uiState() != uiList {
			t.Errorf("%s: state is %s", k, w.m.uiState())
		}
	}
	enter(t, w.m)
	if w.opened != 0 {
		t.Error("the editor opened")
	}
	if s := w.screen(); !strings.Contains(s, "D delete 1") || strings.Contains(s, "enter edit") {
		t.Errorf("the key line:\n%s", s)
	}
}

func TestAGistGoneLeavesTheSelection(t *testing.T) {
	w, gs := several(t, 2)
	press(t, w.m, "V")
	w.s.Remove(gs[0].ID)
	press(t, w.m, "R")
	if got := selectedIDs(w.m); !slices.Equal(got, []string{gs[1].ID}) {
		t.Errorf("selected %v", got)
	}
}

func TestAnotherAccountTakesTheSelectionAway(t *testing.T) {
	w, _ := several(t, 2)
	press(t, w.m, " ", "u")
	press(t, w.m, " ")
	w.m.lastSelected = []string{"x"}
	settle(t, w.m, w.m.update(viewerMsg{login: "bob"}))
	if len(w.m.selected) != 0 || w.m.lastSelected != nil {
		t.Errorf("selected %v, last %v", w.m.selected, w.m.lastSelected)
	}
}

func TestDeleteTheSelection(t *testing.T) {
	w, gs := several(t, 5)
	w.s.Fail("DELETE /gists/"+gs[1].ID, http.StatusInternalServerError)
	w.s.Fail("DELETE /gists/"+gs[3].ID, http.StatusNotFound)
	// gs[3] is the second row, newest first: a draft of it must stay
	w.type_("edited\n")
	press(t, w.m, "j")
	enter(t, w.m)
	press(t, w.m, "esc", "g")
	press(t, w.m, "V", "D")
	if w.m.uiState() != uiConfirmDelete {
		t.Fatalf("state is %s", w.m.uiState())
	}
	w.wants("delete 5 gists", "This deletes 5 gists and their revisions on GitHub.", "It cannot be undone.", "delete these 5 gists?")
	press(t, w.m, "esc")
	if len(w.m.selected) != 5 || deletes(w.s) != 0 {
		t.Fatalf("esc: selected %d, %d deletes", len(w.m.selected), deletes(w.s))
	}
	press(t, w.m, "D", "y")
	w.wants("deleted 3 of 5 · 2 failed · u selects them again · HTTP ")
	for i, g := range gs {
		gone := w.s.Get(g.ID) == nil
		if gone == (i == 1 || i == 3) {
			t.Errorf("%d: gone is %v", i, gone)
		}
	}
	if len(w.m.tab().gists) != 2 || len(w.m.writing) != 0 || w.m.batch != nil {
		t.Errorf("%d in the list, writing %v", len(w.m.tab().gists), w.m.writing)
	}
	if len(w.m.selected) != 0 {
		t.Errorf("selected %v", w.m.selected)
	}
	if d, ok := w.draft(gs[3].ID, "f3.txt"); !ok || d != "edited\n" {
		t.Errorf("the draft of the gist answered 404 is %q, %v", d, ok)
	}
	kept, ok, err := cache.Open(filepath.Join(w.m.opt.CacheDir, cache.MineFile), "github.com", "alice").Load()
	if err != nil || !ok || len(kept.Gists) != 2 {
		t.Errorf("the kept list has %d: %v, %v", len(kept.Gists), ok, err)
	}
	press(t, w.m, "u")
	if got, want := selectedIDs(w.m), []string{gs[1].ID, gs[3].ID}; !slices.Equal(got, want) {
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("selected %v", got)
		}
	}
}

func TestADeletedSelectionIsAllGone(t *testing.T) {
	w, _ := several(t, 3)
	press(t, w.m, "V", "D", "y")
	w.wants("✓ deleted 3 gists", "no gists yet")
	if deletes(w.s) != 3 || w.m.canRestore() {
		t.Errorf("%d deletes", deletes(w.s))
	}
}

func TestARateLimitStopsTheRest(t *testing.T) {
	w, _ := several(t, 3)
	first, _ := w.m.current()
	w.s.Fail("DELETE /gists/"+first.ID, http.StatusTooManyRequests)
	press(t, w.m, "V", "D", "y")
	w.wants("deleted 0 of 3 · 1 failed, 2 not sent · u selects them again · HTTP 429")
	if deletes(w.s) != 1 || len(w.m.writing) != 0 {
		t.Fatalf("%d deletes, writing %v", deletes(w.s), w.m.writing)
	}
	w.s.Fail("DELETE /gists/"+first.ID, 0)
	press(t, w.m, "u", "D", "y")
	// The failure before stays on the line for a while; the list says it
	if deletes(w.s) != 4 || len(w.m.tab().gists) != 0 {
		t.Errorf("%d deletes, %d left", deletes(w.s), len(w.m.tab().gists))
	}
}

func TestAGistBeingSentIsLeftOut(t *testing.T) {
	w, gs := several(t, 2)
	press(t, w.m, "V")
	w.m.writing[gs[0].ID] = "push"
	press(t, w.m, "D")
	w.wants("delete 1 gist", "being sent", "delete this gist?")
	press(t, w.m, "y")
	if w.s.Get(gs[0].ID) == nil || w.s.Get(gs[1].ID) != nil {
		t.Error("the wrong gist went")
	}
	if !w.m.selected[gs[0].ID] {
		t.Error("the gist being sent left the selection")
	}
	w.m.writing[gs[0].ID] = "push"
	press(t, w.m, "D")
	w.wants("wait for what is being sent")
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
}

func TestTheBoxSaysWhatIsHiddenAndWhatHasADraft(t *testing.T) {
	w, gs := several(t, 3)
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc", "V")
	press(t, w.m, "/")
	typeText(t, w.m, "gist 0")
	enter(t, w.m)
	press(t, w.m, "D")
	w.wants("2 are not shown by the filter.", "1 has a draft; it goes too.", draftMark+" draft")
	press(t, w.m, "y")
	if _, ok := w.draft(gs[2].ID, "f2.txt"); ok {
		t.Error("the draft stayed")
	}
}

func TestALongSelectionScrollsInTheBox(t *testing.T) {
	w, _ := several(t, 30)
	press(t, w.m, "V", "D")
	before := w.screen()
	press(t, w.m, "j", "j")
	if w.screen() == before {
		t.Error("j did not scroll")
	}
	for _, size := range [][2]int{{60, 24}, {80, 24}, {30, 10}, {10, 4}} {
		w.m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if lines := strings.Split(w.screen(), "\n"); len(lines) != size[1] {
			t.Errorf("%v: %d lines", size, len(lines))
		}
	}
}

func TestGoldenSelection(t *testing.T) {
	for _, width := range []int{60, 80, 120, 200} {
		m := loaded(t, fixture(), width, 30)
		press(t, m, " ", "j", " ")
		golden(t, fmt.Sprintf("list-selected-%d", width), screen(m))
	}
	ids := regexp.MustCompile(`\b[0-9a-f]{7}\b`)
	ages := regexp.MustCompile(`\b(now|\d+[smhdy]|\d+mo)( ago)?\b`)
	stable := func(w *world) string { return ages.ReplaceAllString(ids.ReplaceAllString(w.screen(), "<id>"), "<age>") }
	w, _ := several(t, 4)
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc", "V", "/")
	typeText(t, w.m, "gist 3")
	enter(t, w.m)
	w.m.writing[w.m.tabs[tabMine].gists[1].ID] = "push"
	press(t, w.m, "D")
	golden(t, "box-delete-many", stable(w))
	long, _ := several(t, 40)
	press(t, long.m, "V", "D")
	golden(t, "box-delete-many-long", stable(long))
}

func TestGistsGoneWhileTheBoxIsOpenAreNotDeleted(t *testing.T) {
	w, gs := several(t, 2)
	press(t, w.m, "V", "D")
	for _, g := range gs {
		w.s.Remove(g.ID)
	}
	settle(t, w.m, w.m.refresh())
	press(t, w.m, "y")
	w.wants("nothing to delete: the gists have left the list")
	if deletes(w.s) != 0 {
		t.Errorf("%d deletes", deletes(w.s))
	}
}

func TestCopyTheURLsOfTheSelection(t *testing.T) {
	w, _ := several(t, 3)
	press(t, w.m, " ", "j", " ")
	cmd := w.m.copyURL()
	if cmd == nil {
		t.Fatal("nothing copied")
	}
	var want []string
	for _, g := range w.m.selectedGists() {
		want = append(want, g.URL)
	}
	if got := fmt.Sprint(cmd()); got != strings.Join(want, "\n") || len(want) != 2 {
		t.Errorf("copied %q, want %q", got, want)
	}
	w.wants("✓ copied 2 URLs")
	if len(w.m.selected) != 2 {
		t.Errorf("selected %v", w.m.selected)
	}
}

func TestOpenTheSelectionInTheBrowser(t *testing.T) {
	w, _ := several(t, maxBrowse+1)
	var opened []string
	w.m.opt.Browse = func(url string) error {
		opened = append(opened, url)
		return nil
	}
	press(t, w.m, "V", "o")
	w.wants(fmt.Sprintf("open at most %d in the browser: %d selected", maxBrowse, maxBrowse+1))
	if len(opened) != 0 {
		t.Fatalf("opened %d", len(opened))
	}
	press(t, w.m, "k", " ", "o")
	if len(opened) != maxBrowse {
		t.Errorf("opened %d", len(opened))
	}
	w.wants(fmt.Sprintf("opening %d in the browser", maxBrowse))
	if want := w.m.selectedGists(); opened[0] != want[0].URL || opened[maxBrowse-1] != want[maxBrowse-1].URL {
		t.Errorf("opened in another order: %v", opened)
	}
	if len(w.m.selected) != maxBrowse {
		t.Errorf("selected %d", len(w.m.selected))
	}
}
