package main

import (
	"os"

	"update-ai-tools/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
