package worker

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"

	"github.com/concourse/concourse/atc/db"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PipelineNameLabelKey       = "concourse/pipeline"
	JobNameLabelKey            = "concourse/job"
	BuildNameLabelKey          = "concourse/build"
	ConcourseNamespace         = "default"
	TaskExecutionContainerName = "task-container"
)

var (
	randReader = rand.Reader

	emptyVolumeSource = v1.VolumeSource{
		EmptyDir: &v1.EmptyDirVolumeSource{},
	}
	// These are injected into all of the source/step containers.
	implicitEnvVars = []v1.EnvVar{{
		Name:  "HOME",
		Value: "/tmp/build",
	}}
	implicitVolumeMounts = []v1.VolumeMount{{
		Name:      "scratch",
		MountPath: "/scratch",
	}, {
		Name:      "home",
		MountPath: "/tmp/build",
	}, {
		Name:      "gcpsecret",
		MountPath: "/secret",
	}}
	implicitVolumes = []v1.Volume{{
		Name:         "scratch",
		VolumeSource: emptyVolumeSource,
	}, {
		Name:         "home",
		VolumeSource: emptyVolumeSource,
	}, {
		Name: "gcpsecret",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: "k8s-blobstore-key-secret",
			},
		},
	}}
)

type pod struct {
	k8sPod v1.Pod
	dbPod  db.Pod
	user   string
	spec   string
}

type Pod interface {
	Stop(kill bool) error
	Info() error
	Run() error
	Attach() error
}

func NewPod(containers []ContainerSpec) Pod {

	return &pod{}
}

func (pod *pod) Stop(kill bool) error {
	return nil
}

func (pod *pod) Info() error {
	return nil
}

func (pod *pod) Run() error {
	return nil
}

func (pod *pod) Attach() error {
	return nil
}

type StepRunnerSpec struct {
	Inputs          []interface{}
	OutputLocations string
	Metadata        db.ContainerMetadata
	Credentials     map[string]interface{}
	mainContainer   ContainerSpec
}

type Result struct {
}

// MakePod will build a k8s pod to run a concourse task
func MakePod(config atc.TaskConfig) *v1.Pod {

	initContainers := []v1.Container{}
	regularContainers := []v1.Container{}
	volumes := implicitVolumes
	volumeMounts := implicitVolumeMounts
	podname := fmt.Sprintf("hello-pod-trial-%s", time.Now().Format("150405"))

	for _, input := range config.Inputs {
		newVolume := v1.Volume{
			Name:         input.Name,
			VolumeSource: emptyVolumeSource,
		}

		newVolumeMount := v1.VolumeMount{
			Name:      input.Name,
			MountPath: fmt.Sprintf("/tmp/build/%s", input.Name),
		}

		volumes = append(volumes, newVolume)
		volumeMounts = append(volumeMounts, newVolumeMount)

		d := v1.Container{
			Name:         fmt.Sprintf("fetch-input-%s", input.Name),
			Image:        "google/cloud-sdk",
			Command:      []string{"/bin/sh"},
			Args:         []string{"-c", fmt.Sprintf("gsutil cp -r gs://k8s-runtime-blobstore/%s/* /tmp/build/%s/", input.Name, input.Name)},
			VolumeMounts: volumeMounts,
			Env:          implicitEnvVars,
			WorkingDir:   "/tmp/build",
		}
		initContainers = append(initContainers, d)
	}

	//for _, output := range config.Outputs {
	//	newVolume := v1.Volume{
	//		Name:         output.Name,
	//		VolumeSource: emptyVolumeSource,
	//	}
	//
	//	newVolumeMount := v1.VolumeMount{
	//		Name:      output.Name,
	//		MountPath: fmt.Sprintf("/tmp/build/%s", output.Name),
	//	}
	//
	//	volumes = append(volumes, newVolume)
	//	volumeMounts = append(volumeMounts, newVolumeMount)
	//
	//	d := v1.Container{
	//		Name:         fmt.Sprintf("upload-output-%s", output.Name),
	//		Image:        "krishnasfood/iamwatching",
	//		Command:      []string{"./watcher"},
	//		Args:         []string{fmt.Sprintf("-pod=%s", podname), fmt.Sprintf("-container=%s", TaskExecutionContainerName)},
	//		VolumeMounts: volumeMounts,
	//		Env:          implicitEnvVars,
	//		WorkingDir:   "/watchthis",
	//	}
	//	regularContainers = append(initContainers, d)
	//}

	// container for the task
	c := v1.Container{
		Name: TaskExecutionContainerName,
		//Image:        spec.mainContainer.ImageSpec.ImageURL,
		Image: "ubuntu:xenial",
		//Command:      []string{spec.mainContainer.Path},
		Command: []string{config.Run.Path},
		//Args:         spec.mainContainer.Args,
		Args:         config.Run.Args,
		VolumeMounts: volumeMounts,
		Env:          implicitEnvVars,
		WorkingDir:   "/tmp/build",
		TTY:          true,
		Stdin:        true,
	}
	regularContainers = append(regularContainers, c)

	// if we need to keep the pod around for hijacking, then we need to add
	// a sidecar that will not terminate until we tell it to

	// another sidecar for streaming logs

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ConcourseNamespace,
			Name:      podname,
			// OwnerReferences: []metav1.OwnerReference{
			// 	*metav1.NewControllerRef(taskRun, groupVersionKind),
			// },
			// Annotations: makeAnnotations(taskRun),
			Labels: map[string]string{
				"some-metadata": "whatever",
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			//ServiceAccountName: "k8s-runtime-spike@cf-concourse-production.iam.gserviceaccount.com",
			InitContainers: initContainers,
			Containers:     regularContainers,
			Volumes:        volumes,
		},
	}
}

func MakePodForGet(config atc.TaskConfig) *v1.Pod {

	initContainers := []v1.Container{}
	regularContainers := []v1.Container{}
	volumes := implicitVolumes
	volumeMounts := implicitVolumeMounts
	podname := fmt.Sprintf("hello-pod-trial-%s", time.Now().Format("150405"))

	//for _, output := range config.Outputs {
	//	newVolume := v1.Volume{
	//		Name:         output.Name,
	//		VolumeSource: emptyVolumeSource,
	//	}
	//
	//	newVolumeMount := v1.VolumeMount{
	//		Name:      output.Name,
	//		MountPath: fmt.Sprintf("/tmp/build/%s", output.Name),
	//	}
	//
	//	volumes = append(volumes, newVolume)
	//	volumeMounts = append(volumeMounts, newVolumeMount)
	//
	//	d := v1.Container{
	//		Name:         fmt.Sprintf("upload-output-%s", output.Name),
	//		Image:        "concourse/porter",
	//		Command:      []string{"./watcher"},
	//		Args:         []string{fmt.Sprintf("-pod=%s", podname), fmt.Sprintf("-container=%s", TaskExecutionContainerName)},
	//		VolumeMounts: volumeMounts,
	//		Env:          implicitEnvVars,
	//		WorkingDir:   "/watchthis",
	//	}
	//	regularContainers = append(initContainers, d)
	//}

	stdoutCollectorContainer := v1.Container{
		Name:            "stdout-sidecar",
		Image:           "concourse/porter-dev:dev",
		ImagePullPolicy: "Always",
		Command:         []string{"/opt/porter/logstream/logstream"},
		Args:            []string{"--source-path=/tmp/build/stdout", fmt.Sprintf("--pod-name=%s", podname), fmt.Sprintf("--container-name=%s", TaskExecutionContainerName)},
		VolumeMounts:    volumeMounts,
		Env:             implicitEnvVars,
		WorkingDir:      "/tmp/build",
		TTY:             true,
		Stdin:           true,
	}
	regularContainers = append(regularContainers, stdoutCollectorContainer)
	// "--pod-name", "gcp-push-pod", "--container-name", "hello"
	stderrCollectorContainer := v1.Container{
		Name:            "stderr-sidecar",
		Image:           "concourse/porter-dev:dev",
		ImagePullPolicy: "Always",
		Command:         []string{"/opt/porter/logstream/logstream"},
		Args:            []string{"--source-path=/tmp/build/stderr", fmt.Sprintf("--pod-name=%s", podname), fmt.Sprintf("--container-name=%s", TaskExecutionContainerName)},
		VolumeMounts:    volumeMounts,
		Env:             implicitEnvVars,
		WorkingDir:      "/tmp/build",
		TTY:             true,
		Stdin:           true,
	}
	regularContainers = append(regularContainers, stderrCollectorContainer)
	//atc.Source{"some": "((source-param))"}
	// container for the get task
	c := v1.Container{
		Name:    TaskExecutionContainerName,
		Image:   "concourse/git-resource:latest",
		Command: []string{"/bin/bash"},
		Args: []string{"-c",
			"echo '{\"source\": {\"uri\": \"https://github.com/vito/booklit\"}}' | /opt/resource/in /tmp/build/in >/tmp/build/stdout 2>/tmp/build/stderr"},
		VolumeMounts: volumeMounts,
		Env:          implicitEnvVars,
		WorkingDir:   "/tmp/build",
	}
	regularContainers = append(regularContainers, c)

	// if we need to keep the pod around for hijacking, then we need to add
	// a sidecar that will not terminate until we tell it to

	// another sidecar for streaming logs

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ConcourseNamespace,
			Name:      podname,
			// OwnerReferences: []metav1.OwnerReference{
			// 	*metav1.NewControllerRef(taskRun, groupVersionKind),
			// },
			// Annotations: makeAnnotations(taskRun),
			Labels: map[string]string{
				"some-metadata": "whatever",
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			//ServiceAccountName: "k8s-runtime-spike@cf-concourse-production.iam.gserviceaccount.com",
			InitContainers: initContainers,
			Containers:     regularContainers,
			Volumes:        volumes,
		},
	}
}
