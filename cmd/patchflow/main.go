package main

import (
	"context"
	"os"

	"patchflow/internal/app"
)

func main() {
	opts := app.Options{
		Args:   os.Args[1:],
		WorkDir: ".",
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Config: ".patchflow.yml",
	}
	if err := app.New(opts).Run(context.Background()); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
