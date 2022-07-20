package framework

import (
	"bytes"
	"os/exec"
	"path/filepath"
)

func execAtLocal(cmd string, input []byte, args ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

	if len(input) != 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (f *Framework) Kubectl(args ...string) ([]byte, []byte, error) {
	return execAtLocal(filepath.Join("kubectl"), nil, args...)
}

func (f *Framework) KubectlWithInput(input []byte, args ...string) ([]byte, []byte, error) {
	return execAtLocal(filepath.Join("kubectl"), input, args...)
}
