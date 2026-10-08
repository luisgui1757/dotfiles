package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

const nativeWireLimit = 2 << 20

// A native worker holds the provider lock while its child runs, even after the
// controller dies. It accepts commands only after reporting that it owns the
// lock. A controller dying during startup therefore cannot leave queued work
// which later races a new controller. This is a command lifetime guard, not a
// package journal: provider adapters still prove the actual native transaction.
type nativeCommand struct {
	Operation   string   `json:"operation,omitempty"`
	Program     string   `json:"program"`
	Arguments   []string `json:"arguments"`
	Environment []string `json:"environment,omitempty"`
	Input       []byte   `json:"input,omitempty"`
}

type nativeReply struct {
	Ready    bool   `json:"ready,omitempty"`
	Output   []byte `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

// Exit codes are data: vendor installers use codes such as 3010 to request a
// reboot. Adapters must not recover them by parsing platform-dependent prose.
type nativeCommandError struct {
	Message  string
	ExitCode *int
}

func (err *nativeCommandError) Error() string { return err.Message }

type nativeCommandRecord struct {
	Schema  int          `json:"schema"`
	Command string       `json:"command"`
	Reply   *nativeReply `json:"reply,omitempty"`
}

func (command nativeCommand) validate() error {
	if command.Operation != "" && !operationID.MatchString(command.Operation) {
		return errors.New("invalid native command operation")
	}
	if !filepath.IsAbs(command.Program) || filepath.Clean(command.Program) != command.Program || strings.ContainsRune(command.Program, 0) ||
		len(command.Arguments) > 256 || len(command.Environment) > 128 || len(command.Input) > 1<<20 {
		return errors.New("invalid native command boundary")
	}
	for _, argument := range command.Arguments {
		if len(argument) > 65536 || strings.ContainsRune(argument, 0) {
			return errors.New("invalid native command argument")
		}
	}
	for _, value := range command.Environment {
		key, _, ok := strings.Cut(value, "=")
		if !ok || key == "" || len(value) > 65536 || strings.ContainsRune(value, 0) {
			return errors.New("invalid native command environment")
		}
	}
	return nil
}

// Drain excessive output instead of closing a pipe and accidentally terminating
// a mutating command. Report the exceeded bound after the actual child exits.
type nativeOutput struct {
	data     bytes.Buffer
	exceeded bool
}

func (output *nativeOutput) Write(data []byte) (int, error) {
	n := len(data)
	remaining := (1 << 20) - output.data.Len()
	if n > remaining {
		data = data[:remaining]
		output.exceeded = true
	}
	_, err := output.data.Write(data)
	return n, err
}

// RunNativeWorker is the private entrypoint of the same installed executable.
// It never elevates itself; adapters supply their reviewed native command.
// Do not bind this lifetime to the controller's cancellation context or kill it
// on EOF: a privileged child may survive that kill and still mutate the host.
func RunNativeWorker(directory string, input io.Reader, output io.Writer) (result error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return errors.New("native worker requires an absolute state directory")
	}
	release, err := Lock(directory)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, release()) }()
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(nativeReply{Ready: true}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), nativeWireLimit)
	for scanner.Scan() {
		var request nativeCommand
		if err := Decode(scanner.Bytes(), &request); err != nil {
			return err
		}
		if err := request.validate(); err != nil {
			return err
		}
		reply, err := executeNativeCommand(directory, request)
		if err != nil {
			return err
		}
		if err := encoder.Encode(reply); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// Persist intent before starting a mutating command, and its result before
// replying. Controller death cannot lose Homebrew's operation marker. An
// unfinished intent is uncertain, never permission to run the command twice.
// Only the digest of arguments/environment is retained, not their raw values.
func executeNativeCommand(directory string, request nativeCommand) (nativeReply, error) {
	var reply nativeReply
	var record nativeCommandRecord
	var path string
	if request.Operation != "" {
		hash, err := digest(request)
		if err != nil {
			return reply, err
		}
		path = filepath.Join(directory, "commands", request.Operation+".json")
		data, err := readDocument(path)
		if err == nil {
			if err := Decode(data, &record); err != nil {
				return reply, err
			}
			if record.Schema != 1 || record.Command != hash {
				return reply, errors.New("native command differs from its saved operation")
			}
			if record.Reply == nil {
				return reply, errors.New("native command has unfinished intent; inspect its native operation evidence before recovery")
			}
			if record.Reply.Ready || len(record.Reply.Output) > 1<<20 || record.Reply.ExitCode == nil && record.Reply.Error == "" {
				return reply, errors.New("invalid saved native command result")
			}
			return *record.Reply, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return reply, err
		}
		record = nativeCommandRecord{Schema: 1, Command: hash}
		if err := saveDocument(path, record); err != nil {
			return reply, err
		}
	}
	command := exec.Command(request.Program, request.Arguments...)
	configureNativeProcess(command, false)
	command.Env = append(os.Environ(), request.Environment...)
	command.Stdin = bytes.NewReader(request.Input)
	var captured nativeOutput
	command.Stdout, command.Stderr = &captured, &captured
	err := command.Run()
	if command.ProcessState != nil {
		code := command.ProcessState.ExitCode()
		reply.ExitCode = &code
	}
	if captured.exceeded {
		err = errors.Join(err, errors.New("native command output exceeded 1 MiB; inspect its operation log"))
	}
	reply.Output = captured.data.Bytes()
	if err != nil {
		reply.Error = err.Error()
	}
	if path != "" {
		record.Reply = &reply
		if err := saveDocument(path, record); err != nil {
			return reply, err
		}
	}
	return reply, nil
}

type nativeWorkerClient struct {
	mutex   sync.Mutex
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Scanner
	pipe    io.ReadCloser
	closed  bool
}

// The caller holds the engine lock throughout this session. No request is sent
// before the ready response. Once a command starts, wait for its actual result
// even if the caller cancels; killing its supervisor cannot prove quiescence.
func startNativeWorker(ctx context.Context, executable string, arguments, environment []string, diagnostics io.Writer) (*nativeWorkerClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	command := exec.Command(executable, arguments...)
	configureNativeProcess(command, true)
	command.Env = append(os.Environ(), environment...)
	command.Stderr = diagnostics
	input, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, errors.Join(err, input.Close())
	}
	if err := command.Start(); err != nil {
		return nil, errors.Join(err, input.Close(), output.Close())
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), nativeWireLimit)
	client := &nativeWorkerClient{command: command, input: input, output: scanner, pipe: output}
	reply, err := client.read()
	if err != nil || !reply.Ready || reply.Error != "" || len(reply.Output) != 0 || reply.ExitCode != nil {
		return nil, errors.Join(errors.New("native worker did not acquire its lock"), err, client.close())
	}
	return client, nil
}

func (client *nativeWorkerClient) read() (nativeReply, error) {
	var reply nativeReply
	if !client.output.Scan() {
		return reply, errors.Join(io.ErrUnexpectedEOF, client.output.Err())
	}
	err := Decode(client.output.Bytes(), &reply)
	return reply, err
}

func (client *nativeWorkerClient) run(ctx context.Context, request nativeCommand) ([]byte, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.closed {
		return nil, errors.New("native worker session is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := request.validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(request)
	if err != nil || len(data)+1 >= nativeWireLimit {
		return nil, errors.Join(errors.New("native command exceeds its protocol bound"), err)
	}
	if _, err := client.input.Write(append(data, '\n')); err != nil {
		return nil, err
	}
	reply, err := client.read()
	if err != nil {
		return nil, err
	}
	if reply.Ready {
		return nil, errors.New("unexpected native worker handshake during execution")
	}
	if reply.Error != "" {
		return reply.Output, &nativeCommandError{Message: fmt.Sprintf("native command failed: %s%s", reply.Error, nativeDiagnostic(reply.Output)), ExitCode: reply.ExitCode}
	}
	if reply.ExitCode == nil || *reply.ExitCode != 0 {
		return reply.Output, errors.New("native worker returned an invalid successful result")
	}
	return reply.Output, nil
}

// Provider output is already bounded in durable evidence. Show a short tail in
// the actionable error too, without allowing native terminal control sequences.
func nativeDiagnostic(output []byte) string {
	prefix := "\n"
	if len(output) > 8192 {
		output = output[len(output)-8192:]
		prefix += "[last 8 KiB of captured output]\n"
	}
	text := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, strings.ToValidUTF8(string(output), "?"))
	if text = strings.TrimSpace(text); text == "" {
		return ""
	}
	return prefix + text
}

func (client *nativeWorkerClient) close() error {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.closed {
		return nil
	}
	client.closed = true
	inputErr := client.input.Close()
	_, drainErr := io.Copy(io.Discard, client.pipe)
	return errors.Join(inputErr, drainErr, client.command.Wait())
}
