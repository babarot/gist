package tui

// uiState is what the screen is in, read from the fields that hold it, in the
// order keys are given out: what is over the screen, then the list. Keys, the
// screen and the key line all go by it, and the tests run through every value,
// so a state added here is one the key line and the key list cannot miss.
type uiState int

const (
	uiList         uiState = iota // the list, with info beside the preview
	uiListComments                // the list, with comments beside the preview
	uiHelp                        // the key list
	uiAuthFail                    // no token was found
	numUIStates
)

var uiStateNames = [numUIStates]string{"list", "list, comments", "key list", "no token"}

func (s uiState) String() string { return uiStateNames[s] }

func (m *Model) uiState() uiState {
	switch {
	case m.noToken:
		return uiAuthFail
	case m.helpOpen:
		return uiHelp
	case m.side == sideComments:
		return uiListComments
	}
	return uiList
}

// opensHelp reports whether ? opens the key list: from a pane, not over a box,
// where ? is nothing.
func (s uiState) opensHelp() bool { return s == uiList || s == uiListComments }
