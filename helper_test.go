// build test

package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"
)

func failTest(t *testing.T, expect, got interface{}) {
	t.Fatalf("expected:%v, got:%v", expect, got)
}

func validateType(t *testing.T, a, b interface{}) {
	expect := reflect.TypeOf(a)
	got := reflect.TypeOf(b)
	if expect != got {
		t.Fatalf("expected type:%v, got:%v", expect, got)
	}
}

func validateResult(t *testing.T, expect, got interface{}) {
	if !reflect.DeepEqual(expect, got) {
		t.Fatalf("expected:%v, got:%v", expect, got)
	}
}

func validateBool(t *testing.T, expect, got bool) {
	if expect != got {
		t.Fatalf("expected:%v, got:%v", expect, got)
	}
}
func isFailedError(t *testing.T, expect, got error) bool {
	if expect == nil {
		if got != nil {
			return true
		}
	} else {
		if got == nil {
			return true
		}
		if got.Error() != expect.Error() {
			return true
		}
	}
	return false
}
func validateError(t *testing.T, expect, got error) {
	if expect == nil {
		if got != nil {
			t.Fatalf("expected:%v, got:%v", nil, got)
		}
	} else {
		if got == nil {
			t.Fatalf("expected:%v, got:%v", expect, nil)
		} else {
			if got.Error() != expect.Error() {
				t.Fatalf("expected:%v, got:%v", got.Error(), expect.Error())
			}
		}
	}
}

func createTestContext(tm time.Duration) (context.Context, context.CancelFunc) {
	if tm == 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), tm)
}

func createTestCommand(ctx context.Context, name string, args ...interface{}) *Command {

	cmd, err := NewCommand(ctx, "bash", args...)
	if err != nil {
		log.Fatalln(err)
	}
	return cmd
}

type shellScript struct {
	StepSleep      float32
	Count          int
	Script         string
	ExpectedStdout []string
	ExpectedStderr []string
	ExitCode       int
}

func testScript() *shellScript {
	script := &shellScript{
		ExpectedStdout: make([]string, 0),
		ExpectedStderr: make([]string, 0),
	}
	return script
}
func (sc *shellScript) CounterLoop(sleep float32, count int, exit int) *shellScript {
	sc.Script = fmt.Sprintf("for i in {0..%d}; do if (( $i %s 2 )); then echo $i >&2; else echo $i; fi; sleep %0.2f; done; exit %d", count, "%", sleep, exit)

	for i := 0; i <= count; i++ {
		if i%2 == 0 {
			sc.ExpectedStdout = append(sc.ExpectedStdout, fmt.Sprintf("%d", i))
		} else {
			sc.ExpectedStderr = append(sc.ExpectedStderr, fmt.Sprintf("%d", i))
		}
	}
	return sc
}

type CommandServiceMock struct {
	errStdoutPipe bool
	errStderrPipe bool
	errStart      bool
	errWait       bool
	stdout        string
	stderr        string
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
	return ioutil.NopCloser(strings.NewReader(m.stdout)), nil
}
func (m *CommandServiceMock) StderrPipe() (io.ReadCloser, error) {
	if m.errStderrPipe {
		return nil, errors.New("errStderrPipe")
	}
	return ioutil.NopCloser(strings.NewReader(m.stderr)), nil
}

// func TestCommandServiceMock(t *testing.T) {
//
// 	testCases := []struct {
// 		name      string
// 		mock      *CommandServiceMock
// 		errStart  error
// 		errStdout error
// 		errStderr error
// 	}{
// 		{name: "default", mock: &CommandServiceMock{errStart: true}, errStart: errors.New("errStart")},
// 		{name: "errStdout", mock: &CommandServiceMock{errStdoutPipe: true}, errStdout: errors.New("errStdoutPipe")},
// 		{name: "errStderr", mock: &CommandServiceMock{errStderrPipe: true}, errStderr: errors.New("errStderrPipe")},
// 	}
// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(tt *testing.T) {
//
// 			if tc.errStart != nil {
// 				if tc.errStart.Error() != tc.mock.Start().Error() {
// 					tt.Fatalf("expected error:%s, got:%s", tc.errStart.Error(), tc.mock.Start().Error())
// 				}
// 			}
// 			if tc.errStdout != nil {
// 				_, errStdout := tc.mock.StdoutPipe()
// 				if tc.errStdout.Error() != errStdout.Error() {
// 					tt.Fatalf("expected error:%s, got:%s", tc.errStdout.Error(), errStdout.Error())
// 				}
// 			}
// 			if tc.errStderr != nil {
// 				_, errStderr := tc.mock.StderrPipe()
// 				if tc.errStderr.Error() != errStderr.Error() {
// 					tt.Fatalf("expected error:%s, got:%s", tc.errStderr.Error(), errStderr.Error())
// 				}
// 			}
// 		})
// 	}
// }
