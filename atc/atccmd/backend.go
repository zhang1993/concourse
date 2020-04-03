package atccmd

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/lockrunner"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/syslog"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/retryhttp"
	"github.com/tedsuo/ifrit/grouper"
)

type BackendCommand struct {
}

func (cmd *BackendCommand) constructMembers(
	logger lager.Logger,
	backendConn db.Conn,
	gcConn db.Conn,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
) ([]grouper.Member, error) {

	backendMembers, err := cmd.constructBackendMembers(logger, backendConn, lockFactory, secretManager)
	if err != nil {
		return nil, err
	}

	gcMembers, err := cmd.constructGCMember(logger, gcConn, lockFactory)
	if err != nil {
		return nil, err
	}

	return append(backendMembers, gcMembers...), nil
}

func (cmd *BackendCommand) constructBackendMembers(
	logger lager.Logger,
	dbConn db.Conn,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
) ([]grouper.Member, error) {

	if cmd.Syslog.Address != "" && cmd.Syslog.Transport == "" {
		return nil, fmt.Errorf("syslog Drainer is misconfigured, cannot configure a drainer without a transport")
	}

	syslogDrainConfigured := true
	if cmd.Syslog.Address == "" {
		syslogDrainConfigured = false
	}

	teamFactory := db.NewTeamFactory(
		dbConn,
		lockFactory,
	)

	resourceFactory := resource.NewResourceFactory()

	dbResourceCacheFactory := db.NewResourceCacheFactory(
		dbConn,
		lockFactory,
	)

	fetchSourceFactory := worker.NewFetchSourceFactory(
		dbResourceCacheFactory,
	)

	resourceFetcher := worker.NewFetcher(
		clock.NewClock(),
		lockFactory,
		fetchSourceFactory,
	)

	dbResourceConfigFactory := db.NewResourceConfigFactory(
		dbConn,
		lockFactory,
	)

	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
	)

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(
		dbConn,
	)

	dbTaskCacheFactory := db.NewTaskCacheFactory(
		dbConn,
	)

	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(
		dbConn,
	)

	dbVolumeRepository := db.NewVolumeRepository(
		dbConn,
	)

	dbWorkerFactory := db.NewWorkerFactory(
		dbConn,
	)

	workerVersion, err := cmd.workerVersion()
	if err != nil {
		return nil, err
	}

	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
		cmd.GardenRequestTimeout,
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider)

	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	buildContainerStrategy, err := cmd.chooseBuildContainerStrategy()
	if err != nil {
		return nil, err
	}

	engine := cmd.constructEngine(
		pool,
		workerClient,
		resourceFactory,
		teamFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		secretManager,
		defaultLimits,
		buildContainerStrategy,
		lockFactory,
	)

	dbBuildFactory := db.NewBuildFactory(
		dbConn,
		lockFactory,
		cmd.GC.OneOffBuildGracePeriod,
	)

	dbCheckFactory := db.NewCheckFactory(
		dbConn,
		lockFactory,
		secretManager,
		cmd.varSourcePool,
		cmd.GlobalResourceCheckTimeout,
	)

	dbPipelineFactory := db.NewPipelineFactory(
		dbConn,
		lockFactory,
	)

	dbJobFactory := db.NewJobFactory(
		dbConn,
		lockFactory,
	)

	componentFactory := db.NewComponentFactory(dbConn)

	err = cmd.configureComponentIntervals(componentFactory)
	if err != nil {
		return nil, err
	}

	alg := algorithm.New(db.NewVersionsDB(dbConn, algorithmLimitRows, schedulerCache))
	bus := dbConn.Bus()

	members := []grouper.Member{
		{Name: "lidar", Runner: lidar.NewRunner(
			logger.Session("lidar"),
			clock.NewClock(),
			lidar.NewScanner(
				logger.Session(atc.ComponentLidarScanner),
				dbCheckFactory,
				secretManager,
				cmd.GlobalResourceCheckTimeout,
				cmd.ResourceCheckingInterval,
			),
			lidar.NewChecker(
				logger.Session(atc.ComponentLidarChecker),
				dbCheckFactory,
				engine,
			),
			cmd.ComponentRunnerInterval,
			bus,
			lockFactory,
			componentFactory,
		)},
		{Name: atc.ComponentScheduler, Runner: scheduler.NewIntervalRunner(
			logger.Session("scheduler-interval-runner"),
			clock.NewClock(),
			lockFactory,
			componentFactory,
			scheduler.NewRunner(
				logger.Session("scheduler"),
				dbJobFactory,
				&scheduler.Scheduler{
					Algorithm: alg,
					BuildStarter: scheduler.NewBuildStarter(
						factory.NewBuildFactory(
							atc.NewPlanFactory(time.Now().Unix()),
						),
						alg),
				},
				cmd.JobSchedulingMaxInFlight,
			),
			cmd.ComponentRunnerInterval,
		)},
		{Name: atc.ComponentBuildTracker, Runner: builds.NewRunner(
			logger.Session("tracker-runner"),
			clock.NewClock(),
			builds.NewTracker(
				logger.Session(atc.ComponentBuildTracker),
				dbBuildFactory,
				engine,
			),
			cmd.ComponentRunnerInterval,
			bus,
			lockFactory,
			componentFactory,
		)},
		// run separately so as to not preempt critical GC
		{Name: atc.ComponentBuildReaper, Runner: lockrunner.NewRunner(
			logger.Session(atc.ComponentBuildReaper),
			gc.NewBuildLogCollector(
				dbPipelineFactory,
				500,
				gc.NewBuildLogRetentionCalculator(
					cmd.DefaultBuildLogsToRetain,
					cmd.MaxBuildLogsToRetain,
					cmd.DefaultDaysToRetainBuildLogs,
					cmd.MaxDaysToRetainBuildLogs,
				),
				syslogDrainConfigured,
			),
			atc.ComponentBuildReaper,
			lockFactory,
			componentFactory,
			clock.NewClock(),
			cmd.ComponentRunnerInterval,
		)},
	}

	if syslogDrainConfigured {
		members = append(members, grouper.Member{
			Name: atc.ComponentSyslogDrainer, Runner: lockrunner.NewRunner(
				logger.Session(atc.ComponentSyslogDrainer),
				syslog.NewDrainer(
					cmd.Syslog.Transport,
					cmd.Syslog.Address,
					cmd.Syslog.Hostname,
					cmd.Syslog.CACerts,
					dbBuildFactory,
				),
				atc.ComponentSyslogDrainer,
				lockFactory,
				componentFactory,
				clock.NewClock(),
				cmd.ComponentRunnerInterval,
			)},
		)
	}
	if cmd.Worker.GardenURL.URL != nil {
		members = cmd.appendStaticWorker(logger, dbWorkerFactory, members)
	}
	return members, nil
}

func (cmd *BackendCommand) constructGCMember(
	logger lager.Logger,
	gcConn db.Conn,
	lockFactory lock.LockFactory,
) ([]grouper.Member, error) {
	var members []grouper.Member

	componentFactory := db.NewComponentFactory(gcConn)
	dbWorkerLifecycle := db.NewWorkerLifecycle(gcConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(gcConn)
	dbContainerRepository := db.NewContainerRepository(gcConn)
	dbArtifactLifecycle := db.NewArtifactLifecycle(gcConn)
	dbCheckLifecycle := db.NewCheckLifecycle(gcConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(gcConn)
	dbBuildFactory := db.NewBuildFactory(gcConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
	dbResourceConfigFactory := db.NewResourceConfigFactory(gcConn, lockFactory)

	dbVolumeRepository := db.NewVolumeRepository(gcConn)

	collectors := map[string]lockrunner.Task{
		atc.ComponentCollectorBuilds:            gc.NewBuildCollector(dbBuildFactory),
		atc.ComponentCollectorWorkers:           gc.NewWorkerCollector(dbWorkerLifecycle),
		atc.ComponentCollectorResourceConfigs:   gc.NewResourceConfigCollector(dbResourceConfigFactory),
		atc.ComponentCollectorResourceCaches:    gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorResourceCacheUses: gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorArtifacts:         gc.NewArtifactCollector(dbArtifactLifecycle),
		atc.ComponentCollectorChecks:            gc.NewCheckCollector(dbCheckLifecycle, cmd.GC.CheckRecyclePeriod),
		atc.ComponentCollectorVolumes:           gc.NewVolumeCollector(dbVolumeRepository, cmd.GC.MissingGracePeriod),
		atc.ComponentCollectorContainers:        gc.NewContainerCollector(dbContainerRepository, cmd.GC.MissingGracePeriod, cmd.GC.HijackGracePeriod),
		atc.ComponentCollectorCheckSessions:     gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle),
		atc.ComponentCollectorVarSources:        gc.NewCollectorTask(cmd.varSourcePool.(gc.Collector)),
	}

	for collectorName, collector := range collectors {
		members = append(members, grouper.Member{
			Name: collectorName, Runner: lockrunner.NewRunner(
				logger.Session(collectorName),
				collector,
				collectorName,
				lockFactory,
				componentFactory,
				clock.NewClock(),
				cmd.ComponentRunnerInterval,
			)},
		)
	}

	return members, nil
}

func (cmd *BackendCommand) constructEngine(
	workerPool worker.Pool,
	workerClient worker.Client,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	secretManager creds.Secrets,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
) engine.Engine {

	stepFactory := builder.NewStepFactory(
		workerPool,
		workerClient,
		resourceFactory,
		teamFactory,
		resourceCacheFactory,
		resourceConfigFactory,
		defaultLimits,
		strategy,
		lockFactory,
	)

	stepBuilder := builder.NewStepBuilder(
		stepFactory,
		builder.NewDelegateFactory(),
		cmd.ExternalURL.String(),
		secretManager,
		cmd.varSourcePool,
		cmd.EnableRedactSecrets,
	)

	return engine.NewEngine(stepBuilder)
}
