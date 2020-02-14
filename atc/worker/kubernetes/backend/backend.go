// backend is responsible for implementing a minimal garden backend server
// that's responsible for creating containers in k8s.
//
package backend

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/garden"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const HandleLabelKey = "tips.ops.handle"

type Backend struct {
	ns  string
	cfg *rest.Config

	cs *kubernetes.Clientset
}

func New(namespace string, config *rest.Config) (backend *Backend, err error) {
	backend = &Backend{
		ns:  namespace,
		cfg: config,
	}

	backend.cs, err = kubernetes.NewForConfig(config)
	if err != nil {
		err = fmt.Errorf("clientset: %w", err)
		return
	}

	return
}

func (b *Backend) Destroy(handle string) (err error) {
	err = b.cs.CoreV1().Pods(b.ns).Delete(handle, &metav1.DeleteOptions{
		GracePeriodSeconds: int64Ref(10),
	})
	if err != nil {
		err = fmt.Errorf("destroy: %w", err)
		return
	}

	return
}

func (b *Backend) waitForPod(ctx context.Context, handle string) (err error) {
	watch, err := b.cs.CoreV1().Pods(b.ns).Watch(metav1.ListOptions{
		LabelSelector: HandleLabelKey + "=" + handle,
	})
	if err != nil {
		err = fmt.Errorf("pods watch: %w", err)
		return
	}

	statusC := make(chan struct{})

	go func() {
		for event := range watch.ResultChan() {
			p, ok := event.Object.(*apiv1.Pod)
			if !ok {
				// TODO show err
				return
			}

			if p.Status.Phase != apiv1.PodRunning {
				continue
			}

			close(statusC)
			return
		}
	}()

	// TODO re-sync on an interval (just because)

	select {
	case <-statusC:
		return
	case <-ctx.Done():
		watch.Stop()
		err = ctx.Err()
		return
	}
}

func (b *Backend) Create(spec garden.ContainerSpec) (container garden.Container, err error) {
	podDefinition := toPod(spec)

	_, err = b.cs.CoreV1().Pods(b.ns).Create(podDefinition)
	if err != nil {
		err = fmt.Errorf("pod creation: %w", err)
		return
	}

	err = b.waitForPod(context.TODO(), spec.Handle)
	if err != nil {
		err = fmt.Errorf("wait for pod: %w", err)
		return
	}

	container = NewContainer(b.ns, spec.Handle, "step", b.cs, b.cfg)

	return
}

const (
	baggageclaimImage      = "cirocosta/baggageclaim"
	baggageclaimVolumeName = "baggageclaim"
)

func bcContainer() apiv1.Container {
	return apiv1.Container{
		Name:  "baggageclaim",
		Image: baggageclaimImage,
		Command: []string{
			"baggageclaim", "--volumes=/tmp", "--driver=naive",
		},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      baggageclaimVolumeName,
				MountPath: "/vols",
			},
		},
	}
}

func bcVolume() apiv1.Volume {
	return apiv1.Volume{
		Name: baggageclaimVolumeName,
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{},
		},
	}
}

func toPod(spec garden.ContainerSpec) (pod *apiv1.Pod) {
	pod = &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.Handle,
			Labels: map[string]string{
				HandleLabelKey: spec.Handle,
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				bcContainer(),
				{
					Name:    "step",
					Image:   spec.Image.URI,
					Command: []string{"/pause"},
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "pause",
							MountPath: "/pause",
							SubPath:   "pause",
						},
					},
				},
			},
			Volumes: []apiv1.Volume{
				{
					Name: "pause",
					VolumeSource: apiv1.VolumeSource{
						ConfigMap: &apiv1.ConfigMapVolumeSource{
							LocalObjectReference: apiv1.LocalObjectReference{"pause"},
							DefaultMode:          int32Ref(0755),
						},
					},
				},
			},
		},
	}

	return
}

func int32Ref(i int32) *int32 {
	return &i
}

func int64Ref(i int64) *int64 {
	return &i
}
