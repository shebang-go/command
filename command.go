package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Data defines an interface for reading stdout and sterr.
type Data interface {

	// Stdout returns stdout data
	Stdout() []string

	// Stderr returns stderr data
	Stderr() []string

	// Out returns a combined output of stdout and stderr
	Out() []string
}

// State defines an interface to read the final command state.
type State interface {

	// ExitCode returns the process exit code. It returns -1 if the process has
	// been killed by a signal (see exec.Wait)
	ExitCode() int

	// Error returns the final error if any.
	Error() error
}

// commandState represents the final state of a command execution.
type commandState struct {
	exit int
	err  error
}

func (c *commandState) ExitCode() int { return c.exit }
func (c *commandState) Error() error  { return c.err }

type commandResult struct {
	stdout []string
	stderr []string
}

func (r *commandResult) Stdout() []string {
	return r.stdout
}

func (r *commandResult) Stderr() []string {
	return r.stderr
}

func (r *commandResult) Out() []string {
	streams := []string{}
	streams = append(streams, r.stdout...)
	streams = append(streams, r.stderr...)
	return streams
}

type streamData struct {
	data     string
	isStderr bool
	err      error
}

func newStreamData(data string, err bool) *streamData {
	return &streamData{data: data, isStderr: err}
}

func (s *streamData) Stdout() []string {
	if !s.isStderr {
		return []string{s.data}
	}
	return nil
}

func (s *streamData) Stderr() []string {
	if s.isStderr {
		return []string{s.data}
	}
	return nil
}

func (s *streamData) Out() []string {
	return []string{s.data}
}

func newCommandResult(stdout, stderr []string) *commandResult {
	r := &commandResult{
		stdout: stdout,
		stderr: stderr,
	}
	return r
}

// Event defines an interface for reading command execution events
type Event interface {
	Error() error
	Data() Data
}

type commandEvent struct {
	data Data
	err  error
}

func newCommandEvent(data Data, err error) *commandEvent {
	c := &commandEvent{data: data, err: err}
	return c
}

func (evt *commandEvent) Error() error {
	return evt.err
}
func (evt *commandEvent) Data() Data {
	return evt.data
}

// Option type sets an internal option (possibly obsolote)
type Option func(*Command) error

// commandService defines an interface for command execution.
type commandService interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Wait() error
	Start() error
}

// processState is an interface for getting the process exit code of a process.
type processState interface {
	ExitCode() int
}

type processStateService struct {
	cmd commandService
}

func newProcessState(cmd commandService) processState {
	return &processStateService{cmd: cmd}
}

func (p *processStateService) ExitCode() int {
	return p.cmd.(*exec.Cmd).ProcessState.ExitCode()
	// return p.cmd.(*commandService).cmd.ProcessState.ExitCode()
}

// Command is a thin wrapper around exec.CommandContext which provides command
// execution using channels only and the ability to stream command output.
// It might be useful in scenarios like a back end service where you want to
// execute workload concurrently.
type Command struct {
	name         string
	args         []string
	outEvents    <-chan Event
	processState processState
	cmd          commandService
	readDone     chan struct{}
	stream       bool
	finalState   chan State
	ctx          context.Context // nil means none
}

// NewCommand returns a new Command object. ctx must be a valid context.Context
// object. The arguments are basically the same as of exec.CommandContext.
// Options can be set using the WithOption(t T) paradigma.
func NewCommand(ctx context.Context, name string, args ...interface{}) (*Command, error) {

	if len(name) == 0 {
		return nil, fmt.Errorf("name cannot be empty")
	}

	cmd := &Command{
		name:       name,
		ctx:        ctx,
		finalState: make(chan State),
		readDone:   make(chan struct{}),

		args: make([]string, 0),
	}
	userOpts := make([]Option, 0)
	var err error
	for _, arg := range args {
		switch v := arg.(type) {
		case Option:
			userOpts = append(userOpts, v)
		case string:
			cmd.args = append(cmd.args, v)
		}
	}
	for _, opt := range userOpts {
		err = opt(cmd)
		if err != nil {
			return nil, err
		}
	}
	if cmd.cmd == nil {
		cmd.cmd = exec.CommandContext(cmd.ctx, cmd.name, cmd.args...)
	}
	cmd.processState = newProcessState(cmd.cmd)
	return cmd, nil
}

// NewCommandStream is the same as NewCommand but enables streaming.
func NewCommandStream(ctx context.Context, name string, args ...interface{}) (*Command, error) {
	cmd, err := NewCommand(ctx, name, args...)
	cmd.stream = true
	return cmd, err
}

// WithStreaming enables streaming of command output
func WithStreaming() Option {

	return func(c *Command) error {
		c.stream = true
		return nil
	}
}
func withCommandService(v commandService) Option {

	return func(c *Command) error {
		c.cmd = v
		return nil
	}
}

func readStream(ctx context.Context, inStream io.Reader, errStream bool) <-chan streamData {
	outStream := make(chan streamData)
	scanner := bufio.NewScanner(inStream)
	var event streamData

	go func() {
		defer close(outStream)
	ForLoop:
		for scanner.Scan() {
			text := scanner.Text()
			event = *newStreamData(text, errStream)
			select {
			case <-ctx.Done():
				break ForLoop
			case outStream <- event:
			}
		}
		if err := scanner.Err(); err != nil {
			event = *newStreamData("", errStream)
			event.err = err
			select {
			case <-ctx.Done():
				return
			case outStream <- event:
			}
		}
	}()
	return outStream
}

func (c *Command) wait() <-chan State {
	go func() {
		<-c.readDone
		err := c.cmd.Wait()
		state := &commandState{err: err}
		if err != nil {
			state.exit = c.processState.ExitCode()
		}
		c.finalState <- state

		defer close(c.finalState)
	}()
	return c.finalState
}

func (c *Command) merge(ctx context.Context, channels ...<-chan streamData) <-chan Event {
	var wg sync.WaitGroup
	mergedStream := make(chan Event)

	multiplex := func(c <-chan streamData) {
		defer wg.Done()
		var event *commandEvent
		for i := range c {
			event = newCommandEvent(newStreamData(i.data, i.isStderr), nil)
			select {
			case <-ctx.Done():
				return
			case mergedStream <- event:
			}
		}
	}

	// merge each channel
	wg.Add(len(channels))
	for _, c := range channels {
		go multiplex(c)
	}

	// Wait for all the reads to complete
	go func() {
		wg.Wait()
		close(c.readDone)
		close(mergedStream)

		// cmd.Wait() must be called after finished reading. See also exec.Wait()
		c.wait()
	}()

	return mergedStream
}

func (c *Command) start() (<-chan Event, error) {
	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.cmd.Start(); err != nil {
		return nil, err
	}
	c.outEvents = c.merge(c.ctx, readStream(c.ctx, stdoutPipe, false), readStream(c.ctx, stderrPipe, true))
	return c.outEvents, nil
}

// Wait must be called after Execute to complete command execution and to
// cleanup resources. It returns a channel which you are required to read from
// to complete the process.
func (c *Command) Wait() <-chan State {
	return c.finalState
}

// Execute starts the command execution. You are required to read from the event
// channel until the channel is closed. The function ensures that all file
// descripters are closed after channel closing.
// If you need to get the final state of the command execution you can read from
// <-command.FinalState.
func (c *Command) Execute() (<-chan Event, error) {
	var event *commandEvent
	var stdout, stderr []string
	outStream := make(chan Event)

	inStream, err := c.start()
	if err != nil {
		return nil, err
	}
	resultReader := func() {
		stdout = []string{}
		stderr = []string{}
	ForLoop:
		for v := range inStream {
			if c.stream {

				select {
				case <-c.ctx.Done():
					break ForLoop

				case outStream <- v:
				}
			} else {
				if len(v.Data().Stderr()) > 0 {
					for _, i := range v.Data().Stderr() {
						stderr = append(stderr, i)
					}
				}
				if len(v.Data().Stdout()) > 0 {
					for _, i := range v.Data().Stdout() {
						stdout = append(stdout, i)
					}
				}
			}
		}
		if !c.stream {
			event = newCommandEvent(newCommandResult(stdout, stderr), errors.New("no error"))
			select {
			case <-c.ctx.Done():
				outStream <- event
				return
			case outStream <- event:
			}
		}
		close(outStream)
	}

	go resultReader()

	return outStream, nil
}
