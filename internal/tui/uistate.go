package tui

// uiState is what the screen is in, read from the fields that hold it, in the
// order keys are given out: what is over the screen, then the list. Keys, the
// screen and the key line all go by it, and the tests run through every value,
// so a state added here is one the key line and the key list cannot miss.
type uiState int

const (
	uiList            uiState = iota // the list, with info beside the preview
	uiListComments                   // the list, with comments beside the preview
	uiHelp                           // the key list
	uiReleaseNotes                   // the release notes, in place of the key list
	uiConfirmPush                    // the diff of an edit
	uiConfirmConflict                // GitHub moved on from an edit
	uiConfirmDelete                  // deleting a gist or a file
	uiConfirmDiscard                 // throwing away a draft
	uiInput                          // typing a description or a file name
	uiInputConflict                  // GitHub moved on from what was typed
	uiFilter                         // typing the filter
	uiNew                            // the form of a new gist
	uiAuthFail                       // no token was found
	numUIStates
)

var uiStateNames = [numUIStates]string{"list", "list, comments", "key list", "release notes", "push", "conflict", "delete",
	"discard", "input", "input conflict", "filter", "new", "no token"}

func (s uiState) String() string { return uiStateNames[s] }

func (m *Model) uiState() uiState {
	switch {
	case m.noToken:
		return uiAuthFail
	case m.form != nil:
		return uiNew
	case m.filtering != nil:
		return uiFilter
	case m.editing != nil && m.editing.conflict != nil:
		return uiConfirmConflict
	case m.editing != nil && m.editing.ready:
		return uiConfirmPush
	case m.input != nil && m.input.conflict != nil:
		return uiInputConflict
	case m.input != nil:
		return uiInput
	case m.deleting != nil:
		return uiConfirmDelete
	case m.discarding != nil:
		return uiConfirmDiscard
	case m.relBox != nil:
		return uiReleaseNotes
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
