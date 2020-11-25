package command

import (
	// "net/url"

	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	_ "os"
	"reflect"
	_ "reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type shellScript struct {
	StepSleep      float32
	Count          int
	ExpectedStdout []string
	ExpectedStderr []string
	ExitCode       int
}

func ShellScript(s float32, c int, exit int) *shellScript {
	script := &shellScript{
		StepSleep:      s,
		Count:          c,
		ExpectedStdout: make([]string, 0),
		ExpectedStderr: make([]string, 0),
		ExitCode:       exit,
	}
	for i := 0; i <= script.Count; i++ {
		if i%2 == 0 {
			script.ExpectedStdout = append(script.ExpectedStdout, fmt.Sprintf("%d", i))
		} else {
			script.ExpectedStderr = append(script.ExpectedStderr, fmt.Sprintf("%d", i))
		}
	}
	return script
}
func (s *shellScript) String() string {
	return fmt.Sprintf("for i in {0..%d}; do if (( $i %s 2 )); then echo $i >&2; else echo $i; fi; sleep %0.2f; done; exit %d", s.Count, "%", s.StepSleep, s.ExitCode)
}

func (s *shellScript) ValidateTest(gotData *TestCaseCommandVariations, wantedData *TestCaseCommandVariations, cmd *Command, t *testing.T) {
	if wantedData.timeout == 0 {
		if !reflect.DeepEqual(s.ExpectedStdout, gotData.stdout) {
			t.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", s.ExpectedStdout, len(s.ExpectedStdout), cap(s.ExpectedStdout), gotData.stdout, len(gotData.stdout), cap(gotData.stdout))
		}

		if !reflect.DeepEqual(s.ExpectedStderr, gotData.stderr) {
			t.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", s.ExpectedStderr, len(s.ExpectedStderr), cap(s.ExpectedStderr), gotData.stderr, len(gotData.stderr), cap(gotData.stderr))
		}
	}
	if wantedData.errCmd != nil {
		if gotData.errCmd != wantedData.errCmd {
			t.Fatalf("expected error:%v, got:%v", wantedData.errCmd, gotData.errCmd)
		}
	} else {
		if gotData.errCmd != nil {
			t.Fatalf("expected no error, got:%v", gotData.errCmd)
		}

	}
	if wantedData.errExec != nil {
		if gotData.errExec != wantedData.errExec {
			t.Fatalf("expected error:%v, got:%v", wantedData.errExec, gotData.errExec)
		}
	} else {
		if gotData.errExec != nil {
			t.Fatalf("expected no error, got:%v", gotData.errExec)
		}

	}
	if wantedData.errCtx != "" {
		if gotData.errCtx != wantedData.errCtx {
			t.Fatalf("expected error:%v, got:%v", wantedData.errCtx, gotData.errCtx)
		}
	} else {
		if gotData.errCtx != "" {
			t.Fatalf("expected no error, got:%v", gotData.errCtx)
		}

	}

	if wantedData.exitCode != gotData.exitCode {
		t.Fatalf("expected exit code:%d, got:%d", wantedData.exitCode, gotData.exitCode)

	}
}

type CommandServiceMock struct {
	errStdoutPipe bool
	errStderrPipe bool
	errStart      bool
	errWait       bool
}

func (m *CommandServiceMock) Start() error {
	if m.errStart {
		return errors.New("errStart")
	}
	return nil
}

func (m *CommandServiceMock) Wait() error {
	if m.errWait {
		return errors.New("errWait")
	}
	return nil
}

func (m *CommandServiceMock) StdoutPipe() (io.ReadCloser, error) {
	if m.errStdoutPipe {
		return nil, errors.New("errStdoutPipe")
	}
	return ioutil.NopCloser(strings.NewReader("test")), nil
}
func (m *CommandServiceMock) StderrPipe() (io.ReadCloser, error) {
	if m.errStderrPipe {
		return nil, errors.New("errStderrPipe")
	}
	return ioutil.NopCloser(strings.NewReader("test")), nil
}

func TestCommandServiceMock(t *testing.T) {

	testCases := []struct {
		name      string
		mock      *CommandServiceMock
		errStart  error
		errStdout error
		errStderr error
	}{
		{name: "default", mock: &CommandServiceMock{errStart: true}, errStart: errors.New("errStart")},
		{name: "errStdout", mock: &CommandServiceMock{errStdoutPipe: true}, errStdout: errors.New("errStdoutPipe")},
		{name: "errStderr", mock: &CommandServiceMock{errStderrPipe: true}, errStderr: errors.New("errStderrPipe")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {

			if tc.errStart != nil {
				if tc.errStart.Error() != tc.mock.Start().Error() {
					tt.Fatalf("expected error:%s, got:%s", tc.errStart.Error(), tc.mock.Start().Error())
				}
			}
			if tc.errStdout != nil {
				_, errStdout := tc.mock.StdoutPipe()
				if tc.errStdout.Error() != errStdout.Error() {
					tt.Fatalf("expected error:%s, got:%s", tc.errStdout.Error(), errStdout.Error())
				}
			}
			if tc.errStderr != nil {
				_, errStderr := tc.mock.StderrPipe()
				if tc.errStderr.Error() != errStderr.Error() {
					tt.Fatalf("expected error:%s, got:%s", tc.errStderr.Error(), errStderr.Error())
				}
			}
		})
	}
}
func TestCommandService(t *testing.T) {

	testCases := []struct {
		name      string
		method    string
		service   *defCommandService
		errWait   error
		errStart  error
		errStdout error
		errStderr error
	}{
		{name: "StdoutPipe", method: "StdoutPipe", service: &defCommandService{cmd: &CommandServiceMock{}}},
		{name: "errStdoutPipe", method: "StdoutPipe", service: &defCommandService{cmd: &CommandServiceMock{errStdoutPipe: true}}, errStdout: errors.New("errStdoutPipe")},
		{name: "StderrPipe", method: "StderrPipe", service: &defCommandService{cmd: &CommandServiceMock{}}},
		{name: "errStderrPipe", method: "StderrPipe", service: &defCommandService{cmd: &CommandServiceMock{errStderrPipe: true}}, errStderr: errors.New("errStderrPipe")},
		{name: "Wait", method: "Wait", service: &defCommandService{cmd: &CommandServiceMock{}}},
		{name: "errWait", method: "Wait", service: &defCommandService{cmd: &CommandServiceMock{errWait: true}}, errWait: errors.New("errWait")},
		{name: "Start", method: "Start", service: &defCommandService{cmd: &CommandServiceMock{}}},
		{name: "errStart", method: "Start", service: &defCommandService{cmd: &CommandServiceMock{errStart: true}}, errStart: errors.New("errStart")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			if tc.method == "StdoutPipe" {
				value, err := tc.service.StdoutPipe()
				if tc.errStdout != nil {
					if err == nil {
						tt.Fatalf("expected error")
					} else if tc.errStdout.Error() != err.Error() {
						tt.Fatalf("expected error:%v, got:%v", tc.errStdout.Error(), err.Error())
					}
				} else {
					if err != nil {
						tt.Fatalf("expected no error, got:%v", err)
					}

				}
				gotType := reflect.TypeOf(&value).Elem()
				wantedType := reflect.TypeOf((*io.ReadCloser)(nil)).Elem()
				if gotType != wantedType {
					t.Fatalf("expected type:%v, got:%v", wantedType, gotType)
				}
			} else if tc.method == "StderrPipe" {
				value, err := tc.service.StderrPipe()
				if tc.errStderr != nil {
					if err == nil {
						tt.Fatalf("expected error")
					} else if tc.errStderr.Error() != err.Error() {
						tt.Fatalf("expected error:%v, got:%v", tc.errStderr.Error(), err.Error())
					}
				} else {
					if err != nil {
						tt.Fatalf("expected no error, got:%v", err)
					}

				}
				gotType := reflect.TypeOf(&value).Elem()
				wantedType := reflect.TypeOf((*io.ReadCloser)(nil)).Elem()
				if gotType != wantedType {
					t.Fatalf("expected type:%v, got:%v", wantedType, gotType)
				}
			} else if tc.method == "Wait" {
				err := tc.service.Wait()
				if tc.errWait != nil {
					if err == nil {
						tt.Fatalf("expected error")
					} else if tc.errWait.Error() != err.Error() {
						tt.Fatalf("expected error:%v, got:%v", tc.errWait.Error(), err.Error())
					}
				} else {
					if err != nil {
						tt.Fatalf("expected no error, got:%v", err)
					}
				}
			} else if tc.method == "Start" {
				err := tc.service.Start()
				if tc.errStart != nil {
					if err == nil {
						tt.Fatalf("expected error")
					} else if tc.errStart.Error() != err.Error() {
						tt.Fatalf("expected error:%v, got:%v", tc.errStart.Error(), err.Error())
					}
				} else {
					if err != nil {
						tt.Fatalf("expected no error, got:%v", err)
					}
				}
			}
		})
	}
}

func TestNewCommand(t *testing.T) {
	cmd, err := NewCommand(context.Background(), "git", "help", "-g")
	if err != nil {
		t.Fatalf("expected no error, got:%v", err)

	}
	expectedArgs := []string{"help", "-g"}
	if !reflect.DeepEqual(cmd.args, expectedArgs) {
		t.Fatalf("expected:%v, got:%v", expectedArgs, cmd.args)
	}

	wantedErr := fmt.Errorf("test error")
	raiseErrorOption := func() commandOption {
		return func(c *Command) error {
			return wantedErr
		}
	}
	_, err = NewCommand(context.Background(), "git", "help", "-g", raiseErrorOption())
	if err == nil {
		t.Fatalf("expected error:%v, got nil", wantedErr)
	}

}

func TestCommandOptions(t *testing.T) {

	cmd, _ := NewCommandStream(context.Background(), "sh", "-c exit")
	if cmd.stream != true {
		t.Fatalf("expected cmd.stream=true")
	}
}

func TestStdoutPipe(t *testing.T) {

	cmd, _ := NewCommandStream(context.Background(), "sh", "-c exit")
	if cmd.stream != true {
		t.Fatalf("expected cmd.stream=true")
	}
}

type ReaderErrorMock struct {
}

func (r *ReaderErrorMock) Read(p []byte) (int, error) {
	return 0, errors.New("errRead")
}
func TestReadStream(t *testing.T) {
	r := strings.NewReader("test")
	stream := readStream(context.Background(), r, false)
	gotValue := <-stream
	expectedValue := "test"
	if gotValue.data != expectedValue {
		t.Fatalf("expected:%s, got:%s", expectedValue, gotValue.data)
	}
}
func TestReadStreamError(t *testing.T) {
	stream := readStream(context.Background(), &ReaderErrorMock{}, false)
	gotValue := <-stream

	if gotValue.err.Error() != "errRead" {
		t.Fatalf("expected error:errRead")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	stream = readStream(ctx, &ReaderErrorMock{}, false)
	cancel()
	select {
	case gotValue = <-stream:
	case <-ctx.Done():
		if ctx.Err().Error() != "context canceled" {
			t.Fatalf("expected error:context canceled")
		}
	}
}

type TestCaseCommandVariations struct {
	name     string
	stream   bool
	errCmd   error
	errExec  error
	errCtx   string
	stdout   []string
	stderr   []string
	timeout  time.Duration
	exitCode int
	script   *shellScript
}

func NewTestCase() *TestCaseCommandVariations {
	t := &TestCaseCommandVariations{
		stdout: make([]string, 0),
		stderr: make([]string, 0),
	}
	return t
}

func TestCommandVariations(t *testing.T) {
	var wg sync.WaitGroup

	testCases := []TestCaseCommandVariations{
		{name: "no-stream", stream: false, script: ShellScript(0.01, 9, 0), exitCode: 0},
		{name: "stream", stream: true, script: ShellScript(0.01, 9, 0), exitCode: 0},
		{name: "no-stream-timeout", stream: false, script: ShellScript(1, 9, 0), exitCode: -1, timeout: 100 * time.Millisecond, errCtx: "context deadline exceeded"},
		{name: "no-stream-exit", stream: false, script: ShellScript(0.01, 9, 10), exitCode: 10},
		{name: "stream-exit", stream: true, script: ShellScript(0.01, 9, 10), exitCode: 10},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			timeout := 10 * time.Second
			if tc.timeout != 0 {
				timeout = tc.timeout
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			var cmd *Command
			var events <-chan CommandEvent
			gotData := NewTestCase()
			if tc.stream {
				wg.Add(1)
				go func() {
					defer wg.Done()
					cmd, gotData.errCmd = NewCommandStream(ctx, "bash", "-c", tc.script.String())
					events, gotData.errExec = cmd.Execute()
					for event := range events {
						gotData.stdout = append(gotData.stdout, event.Data().Stdout()...)
						gotData.stderr = append(gotData.stderr, event.Data().Stderr()...)
					}
				}()
				wg.Wait()

			} else {
				wg.Add(1)
				go func() {
					defer wg.Done()
					cmd, _ = NewCommand(ctx, "bash", "-c", tc.script.String())
					events, _ := cmd.Execute()
					select {
					case <-ctx.Done():
						gotData.errCtx = ctx.Err().Error()
					case event := <-events:
						if event != nil {
							gotData.stdout = event.Data().Stdout()
							gotData.stderr = event.Data().Stderr()
						}
					}
				}()
				wg.Wait()
			}
			state := <-cmd.Wait()
			gotData.exitCode = state.ExitCode()
			tc.script.ValidateTest(gotData, &tc, cmd, tt)
		})
	}
}

func TestCommandPipeErrorStderr(t *testing.T) {
	mock := &CommandServiceMock{
		errStderrPipe: true,
	}
	ctx := context.Background()
	script := ShellScript(0.1, 1, 0)
	cmd, _ := NewCommand(ctx,
		"sh",
		"-c",
		script.String(),
		withCommandService(mock),
	)
	_, err := cmd.start()
	expectedError := errors.New("errStderrPipe")
	if expectedError.Error() != err.Error() {
		t.Fatalf("expected error:%v, got:%v", expectedError, err)
	}
}

func TestCommandStartError(t *testing.T) {
	mock := &CommandServiceMock{
		errStart: true,
	}
	ctx := context.Background()
	script := ShellScript(0.1, 1, 0)
	cmd, _ := NewCommand(ctx,
		"sh",
		"-c",
		script.String(),
		withCommandService(mock),
	)

	_, err := cmd.start()
	expectedError := errors.New("errStart")
	if expectedError.Error() != err.Error() {
		t.Fatalf("expected error:%v, got:%v", expectedError, err)
	}
}

func TestCommandTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	script := ShellScript(0.1, 1, 0)
	cmd, _ := NewCommand(ctx, "sh", "-c", script.String())

	stream, _ := cmd.start()
	expectedError := context.DeadlineExceeded
	var gotError error
ForLoop:
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				break ForLoop
			}
		case <-ctx.Done():
			gotError = ctx.Err()
			break ForLoop
		}
	}
	cancel()
	if expectedError != gotError {
		t.Fatalf("expected error:%v, got:%v", expectedError, gotError)
	}
}

type TestCaseCommandResult struct {
	name          string
	commandResult *commandResult
	expectStdout  []string
	expectStderr  []string
	expectOut     []string
}

func TestCommandResult(t *testing.T) {

	testCases := []TestCaseCommandResult{
		{name: "stdout", commandResult: newCommandResult([]string{"stdout"}, []string{"stderr"}), expectStdout: []string{"stdout"}},
		{name: "stderr", commandResult: newCommandResult([]string{"stdout"}, []string{"stderr"}), expectStderr: []string{"stderr"}},
		{name: "out", commandResult: newCommandResult([]string{"stdout"}, []string{"stderr"}), expectOut: []string{"stdout", "stderr"}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			if tc.expectStdout != nil {
				if !reflect.DeepEqual(tc.expectStdout, tc.commandResult.Stdout()) {
					tt.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", tc.expectStdout, len(tc.expectStdout), cap(tc.expectStdout), tc.commandResult.Stdout(), len(tc.commandResult.Stdout()), cap(tc.commandResult.Stdout()))
				}
			}
			if tc.expectStderr != nil {
				if !reflect.DeepEqual(tc.expectStderr, tc.commandResult.Stderr()) {
					tt.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", tc.expectStderr, len(tc.expectStderr), cap(tc.expectStderr), tc.commandResult.Stderr(), len(tc.commandResult.Stderr()), cap(tc.commandResult.Stderr()))
				}
			}
			if tc.expectOut != nil {
				if !reflect.DeepEqual(tc.expectOut, tc.commandResult.Out()) {
					tt.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", tc.expectOut, len(tc.expectOut), cap(tc.expectOut), tc.commandResult.Out(), len(tc.commandResult.Out()), cap(tc.commandResult.Out()))
				}
			}
		})
	}

}

type TestCaseStreamData struct {
	name         string
	streamData   *streamData
	expectStdout []string
	expectStderr []string
	expectOut    []string
	isError      bool
}

func TestStreamData(t *testing.T) {

	testCases := []TestCaseStreamData{
		{name: "stdout", streamData: newStreamData("stdout", false), expectStdout: []string{"stdout"}},
		{name: "stderr", streamData: newStreamData("stderr", true), expectStderr: []string{"stderr"}, isError: true},
		{name: "out", streamData: newStreamData("data", false), expectOut: []string{"data"}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			if tc.expectStdout != nil {
				if !reflect.DeepEqual(tc.expectStdout, tc.streamData.Stdout()) {
					tt.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", tc.expectStdout, len(tc.expectStdout), cap(tc.expectStdout), tc.streamData.Stdout(), len(tc.streamData.Stdout()), cap(tc.streamData.Stdout()))
				}
			}
			if tc.expectStderr != nil {
				if !reflect.DeepEqual(tc.expectStderr, tc.streamData.Stderr()) {
					tt.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", tc.expectStderr, len(tc.expectStderr), cap(tc.expectStderr), tc.streamData.Stderr(), len(tc.streamData.Stderr()), cap(tc.streamData.Stderr()))
				}
			}
			if tc.expectOut != nil {
				if !reflect.DeepEqual(tc.expectOut, tc.streamData.Out()) {
					tt.Fatalf("expected:%v (l:%d c:%d), got:%v (l:%d c:%d)", tc.expectOut, len(tc.expectOut), cap(tc.expectOut), tc.streamData.Out(), len(tc.streamData.Out()), cap(tc.streamData.Out()))
				}
			}
		})
	}

}

type TestCaseCommandEvent struct {
	name         string
	commandEvent *commandEvent
	err          error
}

func TestCommandEvent(t *testing.T) {

	testCases := []TestCaseCommandEvent{
		{name: "stdout", commandEvent: newCommandEvent(newStreamData("stdout", false), errors.New("test-err")), err: errors.New("test-err")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			if tc.err != nil {
				if tc.err.Error() != tc.commandEvent.Error().Error() {
					tt.Fatalf("expected:%s, got:%s", tc.err.Error(), tc.commandEvent.Error().Error())
				}
			}
		})
	}

}
