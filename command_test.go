// +build !integration
// +build unit

package command

import (
	"context"
	"errors"
	"io"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewCommand(t *testing.T) {
	type testData struct {
		sut *Command
		err error
	}
	testCases := []struct {
		name    string
		args    testData
		varArgs []interface{}
		expect  testData
		got     testData
	}{
		{
			name:   "default",
			args:   testData{sut: &Command{ctx: context.Background(), name: "sh"}},
			expect: testData{sut: &Command{ctx: context.Background(), name: "sh", args: []string{}}},
		},
		{
			name:    "args",
			varArgs: []interface{}{"-c", "exit"},
			args:    testData{sut: &Command{ctx: context.Background(), name: "sh"}},
			expect:  testData{sut: &Command{ctx: context.Background(), name: "sh", args: []string{"-c", "exit"}}},
		},
		{
			name:    "streaming",
			varArgs: []interface{}{"-c", "exit", WithStreaming()},
			args:    testData{sut: &Command{ctx: context.Background(), name: "sh"}},
			expect:  testData{sut: &Command{ctx: context.Background(), name: "sh", stream: true, args: []string{"-c", "exit"}}},
		},
		{
			name:   "nameEmptyError",
			args:   testData{sut: &Command{ctx: context.Background(), name: ""}},
			expect: testData{err: errors.New("name cannot be empty"), sut: &Command{ctx: context.Background(), name: "", args: []string{""}}},
		},
		{
			name:    "optionError",
			varArgs: []interface{}{"-c", "exit", func() Option { return func(c *Command) error { return errors.New("err") } }()},
			args:    testData{sut: &Command{ctx: context.Background(), name: "sh"}},
			expect:  testData{err: errors.New("err")},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			tc.got = testData{}
			tc.got.sut, tc.got.err = NewCommand(tc.args.sut.ctx, tc.args.sut.name, tc.varArgs...)
			validateError(tt, tc.expect.err, tc.got.err)
			if tc.got.err == nil {
				validateResult(tt, tc.expect.sut.name, tc.got.sut.name)
				validateResult(tt, tc.expect.sut.args, tc.got.sut.args)
				validateResult(tt, tc.expect.sut.stream, tc.got.sut.stream)
				validateType(tt, (context.Background()), tc.got.sut.ctx)
			}
		})
	}
}

type ReaderErrorMock struct {
	err    error
	cancel context.CancelFunc
}

func (r *ReaderErrorMock) Read(p []byte) (int, error) {
	if r.cancel != nil {
		log.Println("++++ cancel called")
		r.cancel()
	}
	return 0, r.err
}

func TestCommandReadStream(t *testing.T) {
	type testData struct {
		stream      <-chan streamData
		result      []string
		reader      io.Reader
		isErrStream bool
		errCtx      error
		err         error
	}
	testCases := []struct {
		name     string
		doCancel bool
		args     testData
		timeout  time.Duration
		varArgs  []interface{}
		expect   testData
		got      testData
	}{
		{
			name:   "read",
			args:   testData{reader: strings.NewReader("test")},
			expect: testData{result: []string{"test"}},
		},
		{
			name:   "readError",
			args:   testData{reader: &ReaderErrorMock{err: errors.New("errRead")}},
			expect: testData{err: errors.New("errRead"), result: []string{"test"}},
		},
		{
			name:     "readErrorCancel",
			doCancel: true,
			args:     testData{reader: &ReaderErrorMock{err: errors.New("errRead")}},
			expect:   testData{errCtx: errors.New("context canceled"), result: []string{"test"}},
		},
		{
			name:    "readTimeout",
			timeout: 1 * time.Nanosecond,
			args:    testData{reader: strings.NewReader("test")},
			expect:  testData{errCtx: errors.New("context deadline exceeded"), result: []string{"test"}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ctx, cancel := createTestContext(tc.timeout)
			defer cancel()
			if tc.doCancel {
				switch tc.args.reader.(type) {
				case *ReaderErrorMock:
					tc.args.reader.(*ReaderErrorMock).cancel = cancel
				}
			}
			tc.got = testData{result: make([]string, 0)}
			tc.got.stream = readStream(ctx, tc.args.reader, tc.args.isErrStream)

			var ok bool
			var data streamData
		ForLoop:
			for {
				select {
				case <-ctx.Done():
					tc.got.errCtx = ctx.Err()
					validateError(tt, tc.expect.errCtx, tc.got.errCtx)
					break ForLoop
				case data, ok = <-tc.got.stream:
					if !ok {
						break ForLoop
					}
					tc.got.err = data.err
					tc.got.result = append(tc.got.result, data.data)
				}
			}

			if tc.expect.errCtx == nil {
				validateError(tt, tc.expect.err, tc.got.err)
				if tc.expect.err == nil {
					validateResult(tt, tc.expect.result, tc.got.result)
				}
			}
		})
	}
}

func TestCommandStart(t *testing.T) {
	type testData struct {
		events      <-chan Event
		result      commandResult
		reader      io.Reader
		isErrStream bool
		errCtx      error
		err         error
	}
	type commandArgs struct {
		name   string
		args   []interface{}
		script *shellScript
	}
	testCases := []struct {
		name        string
		argsCommand commandArgs
		timeout     time.Duration
		args        testData
		expect      testData
		got         testData
	}{
		{
			name:   "noerror",
			args:   testData{},
			expect: testData{result: commandResult{stdout: []string{"stdout"}, stderr: []string{"stderr"}}},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{stdout: "stdout", stderr: "stderr"}), "-c"},
			},
		},
		{
			name:    "noerrorTimeout",
			args:    testData{},
			timeout: 1 * time.Nanosecond,
			expect:  testData{errCtx: errors.New("context deadline exceeded"), result: commandResult{stdout: []string{"stdout"}, stderr: []string{"stderr"}}},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{stdout: "stdout", stderr: "stderr"}), "-c"},
			},
		},
		{
			name:   "errStdoutPipe",
			args:   testData{},
			expect: testData{err: errors.New("errStdoutPipe")},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{errStdoutPipe: true}), "-c"},
			},
		},
		{
			name:   "errStderrPipe",
			args:   testData{},
			expect: testData{err: errors.New("errStderrPipe")},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{errStderrPipe: true}), "-c"},
			},
		},
		{
			name:   "errStart",
			args:   testData{},
			expect: testData{err: errors.New("errStart")},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{errStart: true}), "-c"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ctx, cancel := createTestContext(tc.timeout)
			defer cancel()

			var cmd *Command
			cmd = createTestCommand(ctx, tc.argsCommand.name, append(tc.argsCommand.args, tc.argsCommand.script.Script)...)
			tc.got.result = *newCommandResult(make([]string, 0), make([]string, 0))
			tc.got.events, tc.got.err = cmd.start()

			validateError(tt, tc.expect.err, tc.got.err)
			if tc.got.err == nil {
				var ok bool
				var event Event
			ForLoop:
				for {
					select {
					case <-ctx.Done():
						tc.got.errCtx = ctx.Err()
						validateError(tt, tc.expect.errCtx, tc.got.errCtx)
						break ForLoop
					case event, ok = <-tc.got.events:
						if !ok {
							break ForLoop
						}
						tc.got.err = event.Error()
						tc.got.result.stdout = append(tc.got.result.stdout, event.Data().Stdout()...)
						tc.got.result.stderr = append(tc.got.result.stderr, event.Data().Stderr()...)
					default:
					}
				}
				if tc.timeout == 0 {
					validateResult(tt, tc.expect.result.stdout, tc.got.result.stdout)
					validateResult(tt, tc.expect.result.stderr, tc.got.result.stderr)
				}
			}
		})
	}
}

func TestCommandExecute(t *testing.T) {
	type testData struct {
		events      <-chan Event
		result      commandResult
		reader      io.Reader
		isErrStream bool
		errCtx      error
		err         error
	}
	type commandArgs struct {
		name   string
		args   []interface{}
		script *shellScript
	}
	testCases := []struct {
		name        string
		argsCommand commandArgs
		timeout     time.Duration
		args        testData
		expect      testData
		got         testData
	}{
		{
			name:   "noerror",
			args:   testData{},
			expect: testData{result: commandResult{stdout: []string{"stdout"}, stderr: []string{"stderr"}}},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{stdout: "stdout", stderr: "stderr"}), "-c"},
			},
		},
		{
			name:    "noerrorTimeout",
			args:    testData{},
			timeout: 1 * time.Nanosecond,
			expect:  testData{errCtx: errors.New("context deadline exceeded"), result: commandResult{stdout: []string{"stdout"}, stderr: []string{"stderr"}}},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{stdout: "stdout", stderr: "stderr"}), "-c"},
			},
		},
		{
			name:   "errStdoutPipe",
			args:   testData{},
			expect: testData{err: errors.New("errStdoutPipe")},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{errStdoutPipe: true}), "-c"},
			},
		},
		{
			name:   "errStderrPipe",
			args:   testData{},
			expect: testData{err: errors.New("errStderrPipe")},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{errStderrPipe: true}), "-c"},
			},
		},
		{
			name:   "errStart",
			args:   testData{},
			expect: testData{err: errors.New("errStart")},
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 0),
				name:   "bash",
				args:   []interface{}{withCommandService(&CommandServiceMock{errStart: true}), "-c"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ctx, cancel := createTestContext(tc.timeout)
			defer cancel()

			var cmd *Command
			cmd = createTestCommand(ctx, tc.argsCommand.name, append(tc.argsCommand.args, tc.argsCommand.script.Script)...)
			tc.got.result = *newCommandResult(make([]string, 0), make([]string, 0))
			tc.got.events, tc.got.err = cmd.Execute()

			if isFailedError(tt, tc.expect.err, tc.got.err) {
				tt.Fatalf("expected:%v, got:%v", tc.expect.err, tc.got.err)
			}
			if tc.got.err == nil {
				var ok bool
				var event Event
			ForLoop:
				for {
					select {
					case <-ctx.Done():
						tc.got.errCtx = ctx.Err()
						if isFailedError(tt, tc.expect.errCtx, tc.got.errCtx) {
							tt.Fatalf("expected:%v, got:%v", tc.expect.errCtx, tc.got.errCtx)
						}
						break ForLoop
					case event, ok = <-tc.got.events:
						if !ok {
							break ForLoop
						}
						tc.got.err = event.Error()
						tc.got.result.stdout = append(tc.got.result.stdout, event.Data().Stdout()...)
						tc.got.result.stderr = append(tc.got.result.stderr, event.Data().Stderr()...)
					default:
					}
				}
				if tc.timeout == 0 {
					validateResult(tt, tc.expect.result.stdout, tc.got.result.stdout)
					validateResult(tt, tc.expect.result.stderr, tc.got.result.stderr)
				}
			}
		})
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
