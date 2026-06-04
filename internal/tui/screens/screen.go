package screens

type Screen interface {
	Name() string
}

type BaseScreen struct {
	ScreenName string
}

func (s BaseScreen) Name() string { return s.ScreenName }
