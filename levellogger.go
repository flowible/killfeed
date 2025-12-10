package killfeed

import (
	"os"

	"github.com/rs/zerolog"
)

var (
	debugOut = os.Stdout
	errorOut = os.Stderr
)

// logOut implements zerolog.LevelWriter
type LogOut struct{}

// Write should not be called
func (l LogOut) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

// WriteLevel write to the appropriate output
func (l LogOut) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if level < zerolog.WarnLevel {
		return debugOut.Write(p)
	} else {
		return errorOut.Write(p)
	}
}
