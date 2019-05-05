package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	slowBuckets = []float64{
		1, 30, 60, 120, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000,
	}
)

const (
	StatusSucceeded = "succeeded"
	StatusErrored   = "errored"

	LabelStatus = "status"
)

func StatusFromError(err error) string {
	if err != nil {
		return StatusSucceeded
	}

	return StatusErrored
}

var (
	HttpResponseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_http_response_duration_seconds",
			Help: "How long requests are taking to be served.",
		},
		[]string{"code", "route"},
	)

	SchedulingFullDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "concourse_scheduling_full_duration_seconds",
			Help: "How long it took for a full scheduling of a pipeline.",
		},
	)
	SchedulingLoadVersionsDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "concourse_scheduling_loading_versions_duration_seconds",
			Help: "How long it took for a loading versions when scheduling a pipeline.",
		},
	)
	SchedulingJobDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "concourse_scheduling_job_duration_seconds",
			Help: "How long it took for scheduling a single job.",
		},
	)

	ContainersCreationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_containers_creation_duration_seconds",
			Help: "Time taken to create a container",
		},
		[]string{LabelStatus},
	)

	VolumesCreationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_volumes_creation_duration_seconds",
			Help: "Time taken to create a volume",
		},
		[]string{LabelStatus},
	)

	BuildsStarted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concourse_builds_started_total",
			Help: "Number of builds started",
		},
	)
	BuildsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "concourse_builds_duration_seconds",
			Help:    "How long it took for builds to finish",
			Buckets: slowBuckets,
		},
		[]string{LabelStatus},
	)

	ContainersToBeGCed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_gc_containers_to_be_gced_total",
			Help: "Number of containers found for deletion",
		},
		[]string{"type"},
	)
	ContainersGCed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concourse_gc_containers_gced_total",
			Help: "Number containers actually deleted",
		},
	)

	VolumesToBeGCed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_gc_volumes_to_be_gced_total",
			Help: "Number of volumes found for deletion",
		},
		[]string{"type"},
	)
	VolumesGCed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_gc_volumes_gced_total",
			Help: "Number volumes actually deleted",
		},
		[]string{"type"},
	)

	ResourceChecksDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_resource_checks_duration_seconds",
			Help: "How long resource checks take",
		},
		[]string{LabelStatus},
	)

	DatabaseQueries = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concourse_db_queries_total",
			Help: "Number of queries performed",
		},
	)

	// TODO - worker containers
	// TODO - worker volumes
	// TODO - database connections
	// TODO ?
)

func init() {
	prometheus.MustRegister(
		HttpResponseDuration,

		SchedulingFullDuration,
		SchedulingJobDuration,
		SchedulingLoadVersionsDuration,

		ContainersCreationDuration,

		VolumesCreationDuration,

		BuildsStarted,
		BuildsDuration,

		ContainersToBeGCed,
		ContainersGCed,

		VolumesToBeGCed,
		VolumesGCed,

		ResourceChecksDuration,

		DatabaseQueries,
	)
}
