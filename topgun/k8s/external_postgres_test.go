package k8s_test

import (
	"fmt"
	"github.com/onsi/gomega/gexec"
	"path"
	"time"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Prometheus integration", func() {
	var (
		proxySession       *gexec.Session
		releaseName        string
		postgresRelease    string
		prometheusEndpoint string
		namespace          string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-pi-%d-%d", GinkgoRandomSeed(), GinkgoParallelNode())
		namespace = releaseName
		postgresRelease = releaseName + "-prom"

		deployConcourseChart(releaseName,
			"--set=worker.replicas=0",
			"--set=concourse.web.kubernetes.enabled=0",
			"--set=concourse.worker.baggageclaim.driver=detect")

		helmDeploy(postgresRelease,
			namespace,
			path.Join(Environment.ChartsDir, "stable/prometheus"),
			"--set=nodeExporter.enabled=false",
			"--set=kubeStateMetrics.enabled=false",
			"--set=pushgateway.enabled=false",
			"--set=alertmanager.enabled=false",
			"--set=server.persistentVolume.enabled=false")

		waitAllPodsInNamespaceToBeReady(namespace)
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		helmDestroy(postgresRelease)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	It("Is able to retrieve concourse metrics", func() {
		Eventually(func() bool {
			metrics, err := getPrometheusMetrics(prometheusEndpoint, releaseName)
			if err != nil {
				return false
			}

			if metrics.Status != "success" {
				return false
			}

			return true
		}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "be able to retrieve metrics")
	})
})

