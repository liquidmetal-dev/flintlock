# 246. Linters

Status: accepted
Date: 2021-11-10
Authors: @yitsushi
Deciders: @Callisto13 @jmickey @richardcase @yitsushi

# Context

A few linters are disabled and they can improve the code, and the linter tool
(golangci-lint) was misconfigured and a lot of `linters-settings` options were
simply ignored.

# Scope

Enable all linters and set reasonable exception list and linter settings.

# Decision

## gci

> Gci control golang package import order and make it always deterministic.

This makes the import list easier to read and see what imports are from external,
built-in, or internal packages.

### Example

```go
import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)
```

## godox

> Tool for detection of FIXME, TODO and other comment keywords.

No `TODO`, `FIXME`, or `BUG` comments should live in the code without filed
issues to track them. The reason is simple: if we have a comment with "todo",
it has the same value as not having that comment at all, because no one will
care about it.

We can mark block with `TODO`, `FIXME`, or `BUG` if we have a GitHub issues to
fix it. We can annotate these comments with an GitHub reference at the end of
the line.

### Example

```go
// TODO: we may hide this within the firecracker plugin. #179
```

## gochecknoglobals

> A global variable is a variable declared in package scope and that can be
> read and written to by any function within the package. Global variables can
> cause side effects which are difficult to keep track of. A code in one
> function may change the variables state while another unrelated chunk of code
> may be effected by it.

The official description has all the information.

## lll

> Reports long lines

Long lines are hard to read and hard to edit. Most of the time if a line is too
long, it can be reduced with a different pattern (for example Options pattern
for functions), or can be re-formatted into multiple lines.

**Exception:**

If a line contains the `https://` substring, it will be ignored automatically.

## wsl

> Whitespace Linter - Forces you to use empty lines!

This linter has a lot of rules and they make the code easier to read.

### Example

**One big block of code**

<details>
  <summary>Without empty lines</summary>

```go
stdOutFile, err := p.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
if err != nil {
  return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
}
stdErrFile, err := p.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
if err != nil {
  return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
}
cmd.Stderr = stdErrFile
cmd.Stdout = stdOutFile
cmd.Stdin = &bytes.Buffer{}
if !exists {
  if err = p.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
    return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
  }
}
```

</details>

<details>
  <summary>With empty lines</summary>

```go
stdOutFile, err := p.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
if err != nil {
  return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
}

stdErrFile, err := p.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
if err != nil {
  return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
}

cmd.Stderr = stdErrFile
cmd.Stdout = stdOutFile
cmd.Stdin = &bytes.Buffer{}

if !exists {
  if err = p.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
    return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
  }
}
```

</details>

**Error check**

<details>
  <summary>With empty lines</summary>

```go
vmState := NewState(*vmid, p.config.StateRoot, p.fs)
pidPath := vmState.PIDPath()
exists, err := afero.Exists(p.fs, pidPath)
if err != nil {
	return ports.MicroVMStateUnknown, fmt.Errorf("checking pid file exists: %w", err)
}
if !exists {
	return ports.MicroVMStatePending, nil
}
```

</details>

<details>
  <summary>With empty lines</summary>

```go
vmState := NewState(*vmid, p.config.StateRoot, p.fs)
pidPath := vmState.PIDPath()

exists, err := afero.Exists(p.fs, pidPath)
if err != nil {
	return ports.MicroVMStateUnknown, fmt.Errorf("checking pid file exists: %w", err)
}

if !exists {
	return ports.MicroVMStatePending, nil
}
```

</details>

## Description on ignored linters

It is possible to add a `//nolint` command for a specific file, block, or line,
but it's not recommended. If it has a reason why we need that `//nolint`, tell
us a why.

### Example

```go
return rand.New(rand.NewSource(time.Now().UnixNano())) //nolint: gosec // It's not a security context.


//nolint:exhaustivestruct // I don't want to specify all values with nil.
root.AddCommand(&cobra.Command{...})

//nolint:gosec // The purpose of this call is to execute whatever the caller wants.
process := exec.Command(options.Command, options.Args...)
```

# Consequences

- All `todo` comments have a GitHub reference.
- Code will be easier to read and update.
- If a linter rule is ignored, the code itself documents why.
- No unnecessary global variables, less painful debugging what changed that value.
- Spell checker in comments with GB locale. No more `maintanence` or `color`.
- Some of the rules are hard to keep in mind first.

Discussion: https://github.com/liquidmetal-dev/flintlock/discussions/246
