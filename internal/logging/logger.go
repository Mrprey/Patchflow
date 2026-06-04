package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	Dir string
}

func New(dir string) Logger {
	return Logger{Dir: dir}
}

func (l Logger) Write(command string, output string) error {
	if l.Dir == "" {
		return nil
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	name := time.Now().Format("20060102-150405") + ".log"
	path := filepath.Join(l.Dir, name)
	return os.WriteFile(path, []byte(fmt.Sprintf("$ %s\n%s\n", command, output)), 0o600)
}
