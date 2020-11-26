// +build integration

package command

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestIntegrationCommandExecute(t *testing.T) {

	var wg sync.WaitGroup
	type testData struct {
		result commandResult
		state  State
		events <-chan Event
		err    error
		errCtx error
	}
	type commandArgs struct {
		name   string
		args   []interface{}
		script *shellScript
	}
	testCases := []struct {
		name        string
		timeout     time.Duration
		stream      bool
		argsCommand commandArgs
		args        testData
		expect      testData
		got         testData
	}{
		{
			name:   "nostream",
			args:   testData{},
			stream: false,
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 9, 0),
				name:   "bash",
				args:   []interface{}{"-c"},
			},
			expect: testData{state: &commandState{exit: 0}},
		},
		{
			name:   "streaming",
			args:   testData{},
			stream: true,
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 9, 0),
				name:   "bash",
				args:   []interface{}{WithStreaming(), "-c"},
			},
			expect: testData{state: &commandState{exit: 0}},
		},
		{
			name:    "nostreamTimeout",
			args:    testData{},
			timeout: 10 * time.Millisecond,
			stream:  false,
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 9, 0),
				name:   "bash",
				args:   []interface{}{"-c"},
			},
			expect: testData{errCtx: errors.New("context deadline exceeded"), state: &commandState{exit: -1}},
		},
		{
			name:    "streamingTimeout",
			args:    testData{},
			timeout: 10 * time.Millisecond,
			stream:  true,
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 9, 0),
				name:   "bash",
				args:   []interface{}{WithStreaming(), "-c"},
			},
			expect: testData{errCtx: errors.New("context deadline exceeded"), state: &commandState{exit: -1}},
		},
		{
			name:   "nostreamExitCode",
			args:   testData{},
			stream: false,
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 1),
				name:   "bash",
				args:   []interface{}{"-c"},
			},
			expect: testData{errCtx: errors.New("context deadline exceeded"), state: &commandState{exit: 1}},
		},
		{
			name:   "streamingExitCode",
			args:   testData{},
			stream: true,
			argsCommand: commandArgs{
				script: testScript().CounterLoop(0.01, 1, 1),
				name:   "bash",
				args:   []interface{}{"-c"},
			},
			expect: testData{errCtx: errors.New("context deadline exceeded"), state: &commandState{exit: 1}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ctx, cancel := createTestContext(tc.timeout)
			defer cancel()

			var cmd *Command
			if tc.stream {

				wg.Add(1)
				go func() {
					defer wg.Done()
					cmd = createTestCommand(ctx, tc.argsCommand.name, append(tc.argsCommand.args, tc.argsCommand.script.Script)...)
					tc.got.result = *newCommandResult(make([]string, 0), make([]string, 0))
					tc.got.events, tc.got.err = cmd.Execute()
					var ok bool
					var event Event
				ForLoop:
					for {
						select {
						case <-ctx.Done():
							tc.got.errCtx = ctx.Err()
							break ForLoop
						case event, ok = <-tc.got.events:
							if !ok {
								break ForLoop
							}
							tc.got.result.stdout = append(tc.got.result.stdout, event.Data().Stdout()...)
							tc.got.result.stderr = append(tc.got.result.stderr, event.Data().Stderr()...)
						}
					}
				}()
			} else {
				wg.Add(1)
				go func() {
					defer wg.Done()
					cmd = createTestCommand(ctx, tc.argsCommand.name, append(tc.argsCommand.args, tc.argsCommand.script.Script)...)
					tc.got.events, tc.got.err = cmd.Execute()
					select {
					case <-ctx.Done():
						tc.got.errCtx = ctx.Err()
					case event := <-tc.got.events:
						tc.got.result = *newCommandResult(event.Data().Stdout(), event.Data().Stderr())
					}
				}()
			}

			wg.Wait()
			tc.got.state = <-cmd.Wait()
			validateResult(tt, tc.expect.state.ExitCode(), tc.got.state.ExitCode())
			if tc.timeout == 0 {
				validateResult(tt, tc.argsCommand.script.ExpectedStdout, tc.got.result.stdout)
				validateResult(tt, tc.argsCommand.script.ExpectedStderr, tc.got.result.stderr)
			} else {
				validateError(tt, tc.expect.errCtx, tc.got.errCtx)
			}
		})
	}
}
