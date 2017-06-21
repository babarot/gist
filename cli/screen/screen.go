package screen

type Screen struct {
	Lines []Line

	// raw data
	files []File
}

type File struct {
	ID   string
	Name string
}

type Line string

func NewScreen() (s *Screen, err error) {
	var lines []Line
	return &Screen{
		Lines: lines,
	}, nil
}
