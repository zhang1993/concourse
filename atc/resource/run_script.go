package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/concourse/concourse/atc/runner"
	"io"

	"code.cloudfoundry.org/garden"
)

const resourceResultPropertyName = "concourse:resource-result"

const ResourceProcessID = "resource"

type ErrResourceScriptFailed struct {
	Path       string
	Args       []string
	ExitStatus int

	Stderr string
}

func (err ErrResourceScriptFailed) Error() string {
	msg := fmt.Sprintf(
		"resource script '%s %v' failed: exit status %d",
		err.Path,
		err.Args,
		err.ExitStatus,
	)

	if len(err.Stderr) > 0 {
		msg += "\n\nstderr:\n" + err.Stderr
	}

	return msg
}

func (resource *resource) runScript(
	ctx context.Context,
	path string,
	args []string,
	input interface{},
	output interface{},
	logDest io.Writer,
	recoverable bool,
) error {
	request, err := json.Marshal(input)
	if err != nil {
		return err
	}

	if recoverable {
		result, _ := resource.runnable.Properties()
		code := result[resourceResultPropertyName]
		if code != "" {
			return json.Unmarshal([]byte(code), &output)
		}
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	processIO := garden.ProcessIO{
		Stdin:  bytes.NewBuffer(request),
		Stdout: stdout,
	}

	if logDest != nil {
		processIO.Stderr = logDest
	} else {
		processIO.Stderr = stderr
	}

	var process runner.Process

	if recoverable {
		process, err = resource.runnable.Attach(ctx, ResourceProcessID, processIO)
		if err != nil {
			process, err = resource.runnable.Run(
				ctx,
				garden.ProcessSpec{
					ID:   ResourceProcessID,
					Path: path,
					Args: args,
				}, processIO)
			if err != nil {
				return err
			}
		}
	} else {
		process, err = resource.runnable.Run(ctx, garden.ProcessSpec{
			Path: path,
			Args: args,
		}, processIO)
		if err != nil {
			return err
		}
	}

	processExited := make(chan struct{})

	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = process.Wait()
		close(processExited)
	}()

	select {
	case <-processExited:
		if processErr != nil {
			return processErr
		}

		if processStatus != 0 {
			return ErrResourceScriptFailed{
				Path:       path,
				Args:       args,
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		if recoverable {
			err := resource.runnable.SetProperty(resourceResultPropertyName, stdout.String())
			if err != nil {
				return err
			}
		}

		return json.Unmarshal(stdout.Bytes(), output)

	case <-ctx.Done():
		resource.runnable.Stop(false)
		<-processExited
		return ctx.Err()
	}
}
