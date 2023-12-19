/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package exec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/carina-io/carina/utils/log"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Executor is the main interface for all the exec commands
type Executor interface {
	ExecuteCommand(command string, arg ...string) error
	ExecuteCommandWithEnv(env []string, command string, arg ...string) error
	ExecuteCommandWithOutput(command string, arg ...string) (string, error)
	ExecuteCommandWithCombinedOutput(command string, arg ...string) (string, error)
	ExecuteCommandWithOutputFile(command, outfileArg string, arg ...string) (string, error)
	ExecuteCommandWithOutputFileTimeout(timeout time.Duration, command, outfileArg string, arg ...string) (string, error)
	ExecuteCommandWithTimeout(timeout time.Duration, command string, arg ...string) (string, error)
	ExecuteCommandResidentBinary(timeout time.Duration, command string, arg ...string) error
}

// CommandExecutor is the type of the Executor
type CommandExecutor struct {
}

// ExecuteCommand starts a process and wait for its completion
func (c *CommandExecutor) ExecuteCommand(command string, arg ...string) error {
	return c.ExecuteCommandWithEnv([]string{}, command, arg...)
}

// ExecuteCommandWithEnv starts a process with env variables and wait for its completion
func (*CommandExecutor) ExecuteCommandWithEnv(env []string, command string, arg ...string) error {
	cmd, stdout, stderr, err := startCommand(env, command, arg...)
	if err != nil {
		return err
	}

	logOutput(stdout, stderr)

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// ExecuteCommandWithTimeout starts a process and wait for its completion with timeout.
func (*CommandExecutor) ExecuteCommandWithTimeout(timeout time.Duration, command string, arg ...string) (string, error) {
	logCommand(command, arg...)
	// #nosec G204 Rook controls the input to the exec arguments
	cmd := exec.Command(command, arg...)

	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b

	if err := cmd.Start(); err != nil {
		return "", err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	interruptSent := false
	for {
		select {
		case <-time.After(timeout):
			if interruptSent {
				log.Infof("timeout waiting for process %s to return after interrupt signal was sent. Sending kill signal to the process", command)
				var e error
				if err := cmd.Process.Kill(); err != nil {
					log.Errorf("Failed to kill process %s: %v", command, err)
					e = fmt.Errorf("timeout waiting for the command %s to return after interrupt signal was sent. Tried to kill the process but that failed: %v", command, err)
				} else {
					e = fmt.Errorf("timeout waiting for the command %s to return", command)
				}
				return strings.TrimSpace(b.String()), e
			}

			log.Infof("timeout waiting for process %s to return. Sending interrupt signal to the process", command)
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Errorf("Failed to send interrupt signal to process %s: %v", command, err)
				// kill signal will be sent next loop
			}
			interruptSent = true
		case err := <-done:
			if err != nil {
				return strings.TrimSpace(b.String()), err
			}
			if interruptSent {
				return strings.TrimSpace(b.String()), fmt.Errorf("timeout waiting for the command %s to return", command)
			}
			return strings.TrimSpace(b.String()), nil
		}
	}
}

// ExecuteCommandWithOutput executes a command with output
func (*CommandExecutor) ExecuteCommandWithOutput(command string, arg ...string) (string, error) {
	logCommand(command, arg...)
	// #nosec G204 Rook controls the input to the exec arguments
	cmd := exec.Command(command, arg...)
	return runCommandWithOutput(cmd, false)
}

// ExecuteCommandWithCombinedOutput executes a command with combined output
func (*CommandExecutor) ExecuteCommandWithCombinedOutput(command string, arg ...string) (string, error) {
	logCommand(command, arg...)
	// #nosec G204 Rook controls the input to the exec arguments
	cmd := exec.Command(command, arg...)
	return runCommandWithOutput(cmd, true)
}

// ExecuteCommandWithOutputFileTimeout Same as ExecuteCommandWithOutputFile but with a timeout limit.
// #nosec G307 Calling defer to close the file without checking the error return is not a risk for a simple file open and close
func (*CommandExecutor) ExecuteCommandWithOutputFileTimeout(timeout time.Duration,
	command, outfileArg string, arg ...string) (string, error) {

	outFile, err := os.CreateTemp("", "")
	if err != nil {
		return "", fmt.Errorf("failed to open output file: %+v", err)
	}
	defer outFile.Close()
	defer os.Remove(outFile.Name())

	arg = append(arg, outfileArg, outFile.Name())
	logCommand(command, arg...)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// #nosec G204 Rook controls the input to the exec arguments
	cmd := exec.CommandContext(ctx, command, arg...)
	cmdOut, err := cmd.CombinedOutput()

	// if there was anything that went to stdout/stderr then log it, even before
	// we return an error
	if string(cmdOut) != "" {
		log.Debug(string(cmdOut))
	}

	if ctx.Err() == context.DeadlineExceeded {
		return string(cmdOut), ctx.Err()
	}

	if err != nil {
		return string(cmdOut), err
	}

	fileOut, err := io.ReadAll(outFile)
	if err := outFile.Close(); err != nil {
		return "", err
	}
	return string(fileOut), err
}

// ExecuteCommandWithOutputFile executes a command with output on a file
// #nosec G307 Calling defer to close the file without checking the error return is not a risk for a simple file open and close
func (*CommandExecutor) ExecuteCommandWithOutputFile(command, outfileArg string, arg ...string) (string, error) {

	// create a temporary file to serve as the output file for the command to be run and ensure
	// it is cleaned up after this function is done
	outFile, err := os.CreateTemp("", "")
	if err != nil {
		return "", fmt.Errorf("failed to open output file: %+v", err)
	}
	defer outFile.Close()
	defer os.Remove(outFile.Name())

	// append the output file argument to the list or args
	arg = append(arg, outfileArg, outFile.Name())

	logCommand(command, arg...)
	// #nosec G204 Rook controls the input to the exec arguments
	cmd := exec.Command(command, arg...)
	cmdOut, err := cmd.CombinedOutput()
	if err != nil {
		cmdOut = []byte(fmt.Sprintf("%s. %s", string(cmdOut), assertErrorType(err)))
	}
	// if there was anything that went to stdout/stderr then log it, even before we return an error
	if string(cmdOut) != "" {
		log.Debug(string(cmdOut))
	}
	if err != nil {
		return string(cmdOut), err
	}

	// read the entire output file and return that to the caller
	fileOut, err := io.ReadAll(outFile)
	if err := outFile.Close(); err != nil {
		return "", err
	}
	return string(fileOut), err
}

func (*CommandExecutor) ExecuteCommandResidentBinary(timeout time.Duration, command string, arg ...string) error {
	cmd := exec.Command(command, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	go func() {
		if err := cmd.Run(); err != nil {
			log.Errorf("run Resident server failed: %s+v", err)
		}
		//<-c
	}()
	time.Sleep(timeout)
	return nil
}

func startCommand(env []string, command string, arg ...string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	logCommand(command, arg...)

	// #nosec G204 Rook controls the input to the exec arguments
	cmd := exec.Command(command, arg...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Warnf("failed to open stdout pipe: %+v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Warnf("failed to open stderr pipe: %+v", err)
	}

	if len(env) > 0 {
		cmd.Env = env
	}

	err = cmd.Start()

	return cmd, stdout, stderr, err
}

// read from reader line by line and write it to the log
func logFromReader(reader io.ReadCloser) {
	in := bufio.NewScanner(reader)
	lastLine := ""
	for in.Scan() {
		lastLine = in.Text()
		log.Debug(lastLine)
	}
}

func logOutput(stdout, stderr io.ReadCloser) {
	if stdout == nil || stderr == nil {
		log.Warnf("failed to collect stdout and stderr")
		return
	}
	go logFromReader(stderr)
	logFromReader(stdout)
}

func runCommandWithOutput(cmd *exec.Cmd, combinedOutput bool) (string, error) {
	var output []byte
	var err error
	var out string

	if combinedOutput {
		output, err = cmd.CombinedOutput()
	} else {
		output, err = cmd.Output()
		if err != nil {
			output = []byte(fmt.Sprintf("%s. %s", string(output), assertErrorType(err)))
		}
	}

	out = strings.TrimSpace(string(output))

	if err != nil {
		return out, err
	}

	return out, nil
}

func logCommand(command string, arg ...string) {
	log.Debug("Running command: %s %s", command, strings.Join(arg, " "))
}

func assertErrorType(err error) string {
	switch errType := err.(type) {
	case *exec.ExitError:
		return string(errType.Stderr)
	case *exec.Error:
		return errType.Error()
	}

	return ""
}
