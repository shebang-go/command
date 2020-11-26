// +build !integration
// +build example

package command_test

import (
	"context"
	"fmt"

	"github.com/shebang-go/command"
)

func ExampleCommand_Wait() {

	cmd, _ := command.NewCommand(context.Background(), "sh", "-c", "echo hello")

	events, _ := cmd.Execute()
	event := <-events
	fmt.Println(event.Data().Stdout()[0])

	// Wait returns an exit code and error information. Once read from the
	// channel, resources are freed.
	fmt.Println(<-cmd.Wait())

	// Output:
	// hello
	// &{0 <nil>}
}

func ExampleNewCommandStream() {

	cmd, _ := command.NewCommandStream(context.Background(), "sh", "-c", "echo hello")

	events, _ := cmd.Execute()
	for event := range events {
		fmt.Println(event.Data().Out()[0])
	}

	<-cmd.Wait()
	// Output:
	// hello
}

func ExampleNewCommandStream_exitCode() {

	cmd, _ := command.NewCommandStream(context.Background(), "sh", "-c", "echo hello; exit 10")

	events, _ := cmd.Execute()
	for event := range events {
		fmt.Println(event.Data().Out()[0])
	}

	// optionally: read exit code
	result := <-cmd.Wait()
	fmt.Printf("exit code:%d", result.ExitCode())

	// Output:
	// hello
	// exit code:10
}

func ExampleNewCommandStream_stdoutStderr() {

	cmd, _ := command.NewCommandStream(context.Background(), "sh", "-c", "echo data; echo data >&2; exit 10")
	events, _ := cmd.Execute()

	// stdout and stderr may arrive in any order
	for event := range events {

		if event != nil {
			if len(event.Data().Stdout()) > 0 {
				fmt.Println(event.Data().Stdout()[0])
			}

			if len(event.Data().Stderr()) > 0 {
				fmt.Println(event.Data().Stderr()[0])
			}
		}
	}

	// optionally: read exit code
	result := <-cmd.Wait()
	fmt.Printf("exit code:%d", result.ExitCode())

	// Output:
	// data
	// data
	// exit code:10
}
func ExampleCommand() {

	cmd, _ := command.NewCommand(context.Background(), "sh", "-c", "echo stdout; echo stderr >&2; exit 10")
	events, _ := cmd.Execute()

	// get final result
	event := <-events
	if event != nil {
		if len(event.Data().Stdout()) > 0 {
			fmt.Println(event.Data().Stdout()[0])
		}

		if len(event.Data().Stderr()) > 0 {
			fmt.Println(event.Data().Stderr()[0])
		}
	}
	// get final state
	result := <-cmd.Wait()
	fmt.Printf("exit code:%d", result.ExitCode())

	// Output:
	// stdout
	// stderr
	// exit code:10
}
