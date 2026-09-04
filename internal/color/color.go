package color

// ANSI escape sequences for text styles, colors, and terminal control.
const (
	// Reset
	Reset = "\033[0m"

	// Text attributes
	Bold          = "\033[1m"
	Dim           = "\033[2m"
	Underline     = "\033[4m"
	Strikethrough = "\033[9m"

	// Foreground standard colors
	FgBlack   = "\033[30m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgBlue    = "\033[34m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
	FgWhite   = "\033[37m"

	// Foreground bright / grayscale
	FgGray          = "\033[90m"
	FgBrightBlack   = "\033[90m"
	FgBrightRed     = "\033[91m"
	FgBrightGreen   = "\033[92m"
	FgBrightYellow  = "\033[93m"
	FgBrightBlue    = "\033[94m"
	FgBrightMagenta = "\033[95m"
	FgBrightCyan    = "\033[96m"
	FgBrightWhite   = "\033[97m"

	// Foreground bold colors
	FgBoldRed     = "\033[1;31m"
	FgBoldGreen   = "\033[1;32m"
	FgBoldYellow  = "\033[1;33m"
	FgBoldBlue    = "\033[1;34m"
	FgBoldMagenta = "\033[1;35m"
	FgBoldCyan    = "\033[1;36m"
	FgBoldWhite   = "\033[1;37m"

	// Background standard colors
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"

	// Extended 256 colors
	BgDarkGray = "\033[48;5;236m"

	// Compound styles for UI badges and elements
	BadgeVgrep       = "\033[1;30;46m" // Bold black on cyan
	BadgeNormal      = "\033[1;30;42m" // Bold black on green
	BadgeFilter      = "\033[1;30;43m" // Bold black on yellow
	BadgeReplace     = "\033[1;30;44m" // Bold black on blue
	BadgeSearch      = "\033[1;30;46m" // Bold black on cyan
	CursorBlock      = "\033[41;1;37m" // Bold white on red block
	StatusBarBg      = "\033[48;5;236;37m"
	StatusResetBg    = "\033[0;48;5;236;37m"
	StrikethroughDim = "\033[90;9m"

	// Search and pattern highlight styles
	HighlightSearch = "\033[1;31m"
	HighlightFilter = "\033[1;33;4m"
	HighlightMatch  = "\033[1;30;43m"
	HighlightSubst  = "\033[1;30;42m"

	// Terminal screen and cursor controls
	CursorHome     = "\033[H"
	ClearLine      = "\033[K"
	CursorShow     = "\033[?25h"
	CursorHide     = "\033[?25l"
	ClearScreen    = "\033[2J"
	AltScreenEnter = "\033[?1049h"
	AltScreenExit  = "\033[?1049l"
)
