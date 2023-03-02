package crapdav

import (
	"fmt"
	"os"
	"os/exec"
)

func RunEditor (file *os.File) error {
	editor := os.ExpandEnv("$EDITOR")
	if editor == "" {
		return fmt.Errorf("$EDITOR not defined")
	}

	cmd := exec.Command(editor, file.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
