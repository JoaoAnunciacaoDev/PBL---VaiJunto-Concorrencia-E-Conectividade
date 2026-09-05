package enum

type MenuResult int

const (
	MenuContinue MenuResult = iota
	MenuBack
	MenuExit
	MenuDisconnected
)
