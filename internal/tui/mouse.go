package tui

import (
	"slices"

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
	if m.uiState() != uiList {
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
		m.wheel(msg, mouse.Y)
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return nil
		}
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

func (m *Model) wheel(msg tea.MouseWheelMsg, y int) {
	l := m.layout()
	if y < l.listY || y >= l.listY+l.listRows {
		return
	}
	switch msg.Button {
	case tea.MouseWheelDown:
		m.moveTo(m.tab().cur + 1)
	case tea.MouseWheelUp:
		m.moveTo(m.tab().cur - 1)
	}
}

// dragTo sets the preview's height from the row its top edge was dragged to.
func (m *Model) dragTo(y int) {
	l := m.layout()
	m.boxHeight = max(minBoxRows, l.statusY-y)
	m.boxHeight = m.layout().boxH
	m.keepCursorShown()
}

func (m *Model) keepBoxHeight() {
	if m.opt.Layout == nil {
		return
	}
	if err := m.opt.Layout.SavePreviewHeight(m.boxHeight); err != nil {
		m.opt.Log.Warn("the layout cannot be kept", "err", err.Error())
	}
}
