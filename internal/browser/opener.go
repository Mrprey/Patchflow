package browser

import (
	"context"
	"runtime"

	"patchflow/internal/execx"
)

type Opener struct {
	Runner execx.Runner
}

type Interface interface {
	Open(ctx context.Context, url string) error
}

func New(r execx.Runner) Opener {
	if r == nil {
		r = execx.CommandRunner{}
	}
	return Opener{Runner: r}
}

func (o Opener) Open(ctx context.Context, url string) error {
	switch runtime.GOOS {
	case "darwin":
		_, err := o.Runner.Run(ctx, "", "open", url)
		return err
	case "windows":
		_, err := o.Runner.Run(ctx, "", "cmd", "/c", "start", url)
		return err
	default:
		_, err := o.Runner.Run(ctx, "", "xdg-open", url)
		return err
	}
}
