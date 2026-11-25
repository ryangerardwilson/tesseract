// Package player runs the musica command in process. r2d2 runs it as r2d2
// musica.
package player

import (
	"io"

	"github.com/ryangerardwilson/tesseract/internal/musica/cli"
)

// Run runs the musica player command with args. No arguments show the saved
// library in the Tesseract Musica center through the launcher. serve, end,
// play, open, config, help, and download-worker behave as they do on the
// musica binary.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return cli.Main(args, stdin, stdout, stderr)
}
