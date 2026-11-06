package tui

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
)

// hit is a place on the screen a click acts on. They are made as the screen is
// drawn, from the places things were drawn at.
type hit struct {
	y, x0, x1 int
	act       func() tea.Cmd
}

func (m *Model) addHit(y, x0, x1 int, act func() tea.Cmd) {
	m.hits = append(m.hits, hit{y, x0, x1, act})
}

// mouse handles a click, the wheel and dragging the top edge of the preview.
// What is open over the screen keeps the mouse from the list under it.
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.sending {
		return nil
	}
	st := m.uiState()
	if st == uiConfirmPush {
		if w, ok := msg.(tea.MouseWheelMsg); ok {
			if w.Button == tea.MouseWheelDown {
				m.scrollDiff(1)
			} else {
				m.scrollDiff(-1)
			}
		}
		return nil
	}
	if st != uiList && st != uiListComments {
		return nil
	}
	mouse := msg.Mouse()
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		if m.dragging {
			m.dragTo(mouse.Y)
		}
	case tea.MouseReleaseMsg:
		if m.dragging {
			m.dragging = false
			m.keepBoxHeight()
		}
	case tea.MouseWheelMsg:
		m.wheel(msg, mouse.X, mouse.Y)
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return nil
		}
		// A second click on a row soon after edits it
		defer func(row int) { m.lastClick, m.lastRow = m.now(), row }(m.tab().cur)
		// Latest drawn first, as what is drawn later is on top
		for _, h := range slices.Backward(m.hits) {
			if h.y == mouse.Y && mouse.X >= h.x0 && mouse.X < h.x1 {
				return h.act()
			}
		}
		if l := m.layout(); l.boxH > 0 && mouse.Y == l.boxY {
			m.dragging = true
		}
	}
	return nil
}

// wheel moves what is under the mouse: the list, or the content or comments of
// the preview, a line at a time.
// doubleClick is how close two clicks on a row are to edit it.
const doubleClick = 400 * time.Millisecond

func (m *Model) isDoubleClick(row int) bool {
	return m.tab().cur == row && m.lastRow == row && m.now().Sub(m.lastClick) < doubleClick
}

func (m *Model) wheel(msg tea.MouseWheelMsg, x, y int) {
	l := m.layout()
	by := 0
	switch msg.Button {
	case tea.MouseWheelDown:
		by = 1
	case tea.MouseWheelUp:
		by = -1
	}
	switch {
	case y >= l.listY && y < l.listY+l.listRows:
		m.moveTo(m.tab().cur + by)
	case y > l.boxY && y < l.boxY+l.boxH-1:
		if x >= m.paneX() {
			if m.side == sideComments {
				m.scrollComments(by)
			}
			return
		}
		m.scrollContent(by)
	}
}

// dragTo sets the preview's height from the row its top edge was dragged to.
func (m *Model) dragTo(y int) {
	m.setBoxHeight(m.layout().statusY - y)
}

// setBoxHeight sets the preview's height, as far as the terminal has room.
func (m *Model) setBoxHeight(h int) {
	m.boxHeight = max(minBoxRows, h)
	m.boxHeight = m.layout().boxH
	m.keepCursorShown()
}

// resizeBox sets the preview's height and keeps it for the next start.
func (m *Model) resizeBox(h int) {
	was := m.boxHeight
	m.setBoxHeight(h)
	if m.boxHeight != was {
		m.keepBoxHeight()
	}
}

func (m *Model) keepBoxHeight() {
	if m.opt.Layout == nil {
		return
	}
	if err := m.opt.Layout.SavePreviewHeight(m.boxHeight); err != nil {
		m.opt.Log.Warn("the layout cannot be kept", "err", err.Error())
	}
}
