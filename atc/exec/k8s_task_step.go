package exec

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"flag"
	"fmt"
	"io"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	"path/filepath"
	"time"
	"errors"

	"code.cloudfoundry.org/lager"
	boshtemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/worker"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

// TaskStep executes a TaskConfig, whose inputs will be fetched from the
// artifact.Repository and outputs will be added to the artifact.Repository.
type K8sTaskStep struct {
	planID            atc.PlanID
	plan              atc.TaskPlan
	build             db.Build
	defaultLimits     atc.ContainerLimits
	containerMetadata db.ContainerMetadata
	secrets           creds.Secrets
	delegate          TaskDelegate
	succeeded         bool
}

func NewK8sTaskStep(
	planID atc.PlanID,
	plan atc.TaskPlan,
	build db.Build,
	defaultLimits atc.ContainerLimits,
	containerMetadata db.ContainerMetadata,
	secrets creds.Secrets,
	delegate TaskDelegate,
) Step {
	return &K8sTaskStep{
		planID:            planID,
		plan:              plan,
		build:             build,
		defaultLimits:     defaultLimits,
		containerMetadata: containerMetadata,
		secrets:           secrets,
		delegate:          delegate,
	}
}

// Run will first select the worker based on the TaskConfig's platform and the
// TaskStep's tags, and prioritize it by availability of volumes for the TaskConfig's
// inputs. Inputs that did not have volumes available on the worker will be streamed
// in to the container.
//
// If any inputs are not available in the artifact.Repository, MissingInputsError
// is returned.
//
// Once all the inputs are satisfied, the task's script will be executed. If
// the task is canceled via the context, the script will be interrupted.
//
// If the script exits successfully, the outputs specified in the TaskConfig
// are registered with the artifact.Repository. If no outputs are specified, the
// task's entire working directory is registered as an ArtifactSource under the
// name of the task.
func (step *K8sTaskStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("task-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.build.JobID(),
	})

	variables := creds.NewVariables(step.secrets, step.build.TeamName(), step.build.PipelineName())

	var taskConfigSource TaskConfigSource
	var taskVars []boshtemplate.Variables

	if step.plan.ConfigPath != "" {
		// external task - construct a source which reads it from file
		taskConfigSource = FileConfigSource{ConfigPath: step.plan.ConfigPath}

		// for interpolation - use 'vars' from the pipeline, and then fill remaining with cred variables
		taskVars = []boshtemplate.Variables{boshtemplate.StaticVariables(step.plan.Vars), variables}
	} else {
		// embedded task - first we take it
		taskConfigSource = StaticConfigSource{Config: step.plan.Config}

		// for interpolation - use just cred variables
		taskVars = []boshtemplate.Variables{variables}
	}

	// override params
	taskConfigSource = &OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: step.plan.Params}

	// interpolate template vars
	taskConfigSource = InterpolateTemplateConfigSource{ConfigSource: taskConfigSource, Vars: taskVars}

	// validate
	taskConfigSource = ValidatingConfigSource{ConfigSource: taskConfigSource}

	repository := state.Artifacts()

	config, err := taskConfigSource.FetchConfig(logger, repository)

	for _, warning := range taskConfigSource.Warnings() {
		fmt.Fprintln(step.delegate.Stderr(), "[WARNING]", warning)
	}

	if err != nil {
		return err
	}

	if config.Limits.CPU == nil {
		config.Limits.CPU = step.defaultLimits.CPU
	}
	if config.Limits.Memory == nil {
		config.Limits.Memory = step.defaultLimits.Memory
	}

	step.delegate.Initializing(logger, config)

	// connect to k8s API when ATC starts, pass down client
	var kubeconfig *string
	if flag.Lookup("kubeconfig") == nil {
		if home := homeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}

	} else {
		value := flag.Lookup("kubeconfig").Value.String()
		kubeconfig = &value
	}


	flag.Parse()
	// use the current context in kubeconfig
	podConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(podConfig)
	if err != nil {
		panic(err.Error())
	}

	// grab values from K8sTaskStep, Build, some other places
	podSpec := worker.MakePod(config)
	// make a pod
	pod, err := clientset.CoreV1().Pods("default").Create(podSpec)
	if err != nil {
		panic(err.Error())
	}

	// wait for pod to exit with success/fail
	err = WaitForPodState(clientset, podSpec.Name, "default", func(pod *v1.Pod) (bool, error){
		if pod.Status.Phase == "Succeeded" {
			return true, nil
		} else if pod.Status.Phase == "Failed" {
			return true, errors.New("failed")
		}
		return false, nil
	}, "PodIsDone" )
	fmt.Println("Pod is done!!!")

	if err != nil{
		logger.Debug("task-failed")
		step.succeeded = false
		return nil
	}


	step.succeeded = true

	// get logs -- working from fly watch, not working in UI
	for _, c := range pod.Spec.Containers {
		logReader, err := clientset.CoreV1().Pods("default").GetLogs(podSpec.Name, &v1.PodLogOptions{Container: c.Name}).Stream()
		if err != nil{
			panic(err.Error())
		}
		io.Copy(step.delegate.Stdout(), logReader)
	}

	// make a container spec
	fmt.Println("ITS A POD!!!!!!! ======", pod)
	return nil
}

const (
	interval = 1 * time.Second
	timeout  = 10 * time.Minute
)

func WaitForPodState(c *kubernetes.Clientset, name string, namespace string, inState func(r *v1.Pod) (bool, error), desc string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.CoreV1().Pods(namespace).Get(name, v12.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func (step *K8sTaskStep) Succeeded() bool {
	return step.succeeded
}

func (step *K8sTaskStep) imageSpec(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig) (worker.ImageSpec, error) {
	imageSpec := worker.ImageSpec{
		Privileged: bool(step.plan.Privileged),
	}

	// Determine the source of the container image
	// a reference to an artifact (get step, task output) ?
	if step.plan.ImageArtifactName != "" {
		source, found := repository.SourceFor(artifact.Name(step.plan.ImageArtifactName))
		if !found {
			return worker.ImageSpec{}, MissingTaskImageSourceError{step.plan.ImageArtifactName}
		}

		imageSpec.ImageArtifactSource = source

		//an image_resource
	} else if config.ImageResource != nil {
		imageSpec.ImageResource = &worker.ImageResource{
			Type:    config.ImageResource.Type,
			Source:  creds.NewSource(boshtemplate.StaticVariables{}, config.ImageResource.Source),
			Params:  config.ImageResource.Params,
			Version: config.ImageResource.Version,
		}
		// a rootfs_uri
	} else if config.RootfsURI != "" {
		imageSpec.ImageURL = config.RootfsURI
	}

	return imageSpec, nil
}

func (step *K8sTaskStep) containerInputs(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) ([]worker.InputSource, error) {
	inputs := []worker.InputSource{}

	var missingRequiredInputs []string
	for _, input := range config.Inputs {
		inputName := input.Name
		if sourceName, ok := step.plan.InputMapping[inputName]; ok {
			inputName = sourceName
		}

		source, found := repository.SourceFor(artifact.Name(inputName))
		if !found {
			if !input.Optional {
				missingRequiredInputs = append(missingRequiredInputs, inputName)
			}
			continue
		}

		inputs = append(inputs, &taskInputSource{
			config:        input,
			source:        source,
			artifactsRoot: metadata.WorkingDirectory,
		})
	}

	if len(missingRequiredInputs) > 0 {
		return nil, MissingInputsError{missingRequiredInputs}
	}

	for _, cacheConfig := range config.Caches {
		source := newTaskCacheSource(logger, step.build.TeamID(), step.build.JobID(), step.plan.Name, cacheConfig.Path)
		inputs = append(inputs, &taskCacheInputSource{
			source:        source,
			artifactsRoot: metadata.WorkingDirectory,
			cachePath:     cacheConfig.Path,
		})
	}

	return inputs, nil
}

func (step *K8sTaskStep) containerSpec(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) (worker.ContainerSpec, error) {
	imageSpec, err := step.imageSpec(logger, repository, config)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	containerSpec := worker.ContainerSpec{
		Platform:  config.Platform,
		Tags:      step.plan.Tags,
		TeamID:    step.build.TeamID(),
		ImageSpec: imageSpec,
		Limits:    worker.ContainerLimits(config.Limits),
		User:      config.Run.User,
		Dir:       metadata.WorkingDirectory,
		Env:       step.envForParams(config.Params),

		Inputs:  []worker.InputSource{},
		Outputs: worker.OutputPaths{},
	}

	containerSpec.Inputs, err = step.containerInputs(logger, repository, config, metadata)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	for _, output := range config.Outputs {
		path := artifactsPath(output, metadata.WorkingDirectory)
		containerSpec.Outputs[output.Name] = path
	}

	return containerSpec, nil
}

func (step *K8sTaskStep) workerSpec(logger lager.Logger, resourceTypes creds.VersionedResourceTypes, repository *artifact.Repository, config atc.TaskConfig) (worker.WorkerSpec, error) {
	workerSpec := worker.WorkerSpec{
		Platform:      config.Platform,
		Tags:          step.plan.Tags,
		TeamID:        step.build.TeamID(),
		ResourceTypes: resourceTypes,
	}

	imageSpec, err := step.imageSpec(logger, repository, config)
	if err != nil {
		return worker.WorkerSpec{}, err
	}

	if imageSpec.ImageResource != nil {
		workerSpec.ResourceType = imageSpec.ImageResource.Type
	}

	return workerSpec, nil
}

func (step *K8sTaskStep) registerOutputs(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, container worker.Container, metadata db.ContainerMetadata) error {
	volumeMounts := container.VolumeMounts()

	logger.Debug("registering-outputs", lager.Data{"outputs": config.Outputs})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := step.plan.OutputMapping[output.Name]; ok {
			outputName = destinationName
		}

		outputPath := artifactsPath(output, metadata.WorkingDirectory)

		for _, mount := range volumeMounts {
			if filepath.Clean(mount.MountPath) == filepath.Clean(outputPath) {
				source := NewTaskArtifactSource(mount.Volume)
				repository.RegisterSource(artifact.Name(outputName), source)
			}
		}
	}

	// Do not initialize caches for one-off builds
	if step.build.JobID() != 0 {
		logger.Debug("initializing-caches", lager.Data{"caches": config.Caches})

		for _, cacheConfig := range config.Caches {
			for _, volumeMount := range volumeMounts {
				if volumeMount.MountPath == filepath.Join(metadata.WorkingDirectory, cacheConfig.Path) {
					logger.Debug("initializing-cache", lager.Data{"path": volumeMount.MountPath})

					err := volumeMount.Volume.InitializeTaskCache(
						logger,
						step.build.JobID(),
						step.plan.Name,
						cacheConfig.Path,
						bool(step.plan.Privileged))
					if err != nil {
						return err
					}

					continue
				}
			}
		}
	}

	return nil
}

func (K8sTaskStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

