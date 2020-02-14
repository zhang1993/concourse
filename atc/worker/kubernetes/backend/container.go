package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"code.cloudfoundry.org/garden"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	mainContainer = "main"
)

type Container struct {
	ns, pod   string
	clientset *kubernetes.Clientset
	cfg       *rest.Config
}

func NewContainer(ns, pod string, clientset *kubernetes.Clientset, cfg *rest.Config) *Container {
	return &Container{
		ns:        ns,
		pod:       pod,
		clientset: clientset,
		cfg:       cfg,
	}
}

func (c *Container) RunScript(
	ctx context.Context,
	path string,
	args []string,
	input []byte,
	output interface{},
	logDest io.Writer,
	recoverable bool,
) (err error) {
	runtime.Breakpoint()

	procSpec := garden.ProcessSpec{
		Path: path,
		Args: args,
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	procIO := garden.ProcessIO{
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  bytes.NewBuffer(input),
	}

	_, err = c.Run(procSpec, procIO)
	if err != nil {
		err = fmt.Errorf("container run: %w", err)
		return
	}

	err = json.Unmarshal(stdout.Bytes(), output)
	if err != nil {
		err = fmt.Errorf("output unmarshal: %w", err)
		return
	}

	return
}

func (c *Container) Run(procSpec garden.ProcessSpec, procIO garden.ProcessIO) (process garden.Process, err error) {
	sess := log.WithFields(log.Fields{
		"action": "run",
		"ns":     c.ns,
		"pod":    c.pod,
		"cmd":    procSpec.Path,
	})

	sess.Info("start")
	defer sess.Info("finished")

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(c.pod).
		Namespace(c.ns).
		SubResource("exec").
		Param("container", mainContainer)

	req.VersionedParams(&apiv1.PodExecOptions{
		Container: mainContainer,
		Command:   append([]string{procSpec.Path}, procSpec.Args...),
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.cfg, "POST", req.URL())
	if err != nil {
		return
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  procIO.Stdin,
		Stdout: procIO.Stdout,
		Stderr: procIO.Stderr,
	})
	if err != nil {
		return
	}

	return
}

func (c *Container) Handle() (handle string) {
	return
}

func (c *Container) Stop(kill bool) (err error)                                                { return }
func (c *Container) Info() (info garden.ContainerInfo, err error)                              { return }
func (c *Container) StreamIn(spec garden.StreamInSpec) (err error)                             { return }
func (c *Container) StreamOut(spec garden.StreamOutSpec) (readCloser io.ReadCloser, err error) { return }
func (c *Container) CurrentBandwidthLimits() (limits garden.BandwidthLimits, err error)        { return }
func (c *Container) CurrentCPULimits() (limits garden.CPULimits, err error)                    { return }
func (c *Container) CurrentDiskLimits() (limits garden.DiskLimits, err error)                  { return }
func (c *Container) CurrentMemoryLimits() (limits garden.MemoryLimits, err error)              { return }
func (c *Container) NetIn(hostPort, containerPort uint32) (a, b uint32, err error)             { return }
func (c *Container) NetOut(netOutRule garden.NetOutRule) (err error)                           { return }
func (c *Container) BulkNetOut(netOutRules []garden.NetOutRule) (err error)                    { return }
func (c *Container) Metrics() (metrics garden.Metrics, err error)                              { return }
func (c *Container) SetGraceTime(graceTime time.Duration) (err error)                          { return }
func (c *Container) Properties() (properties garden.Properties, err error)                     { return }
func (c *Container) Property(name string) (value string, err error)                            { return }
func (c *Container) SetProperty(name string, value string) (err error)                         { return }
func (c *Container) RemoveProperty(name string) (err error)                                    { return }
func (c *Container) Attach(processID string, io garden.ProcessIO) (process garden.Process, err error) {
	return
}
