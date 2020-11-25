# command module

This module is a small wrapper for the standard library's exec package. It
provides command execution using channels only and the ability to stream
command output. It might be useful in scenarios like a back end service
where you want to execute workload concurrently.

Example

```go
	cmd, _ := command.NewCommand(context.Background(), "sh", "-c", "echo hello")

	events, _ := cmd.Execute()
	event := <-events
	fmt.Println(event.Data().Stdout()[0])

	// Wait returns an exit code and error information. Once read from the
	// channel, resources are freed.
	fmt.Println(<-cmd.Wait())
```
## Badges
[![Release](https://img.shields.io/github/release/shebang-go/command.svg?style=for-the-badge)](https://github.com/shebang-go/command/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](/LICENSE.md)
[![Build status](https://img.shields.io/github/workflow/status/shebang-go/command/build?style=for-the-badge)](https://github.com/shebang-go/command/actions?workflow=build)
[![Codecov](https://img.shields.io/codecov/c/github/shebang-go/command/master.svg?style=for-the-badge)](https://codecov.io/gh/shebang-go/command)

