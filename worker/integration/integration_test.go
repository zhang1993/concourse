package integration_test

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/worker"

	"github.com/jessevdk/go-flags"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	cmd "github.com/concourse/concourse/worker/workercmd"
)

type WorkerSuite struct {
	suite.Suite
	*require.Assertions

	workerCommand *cmd.WorkerCommand
}

func (s *WorkerSuite) SetupSuite() {
	//args := make([]string, 0)
	//baggageClaimCmd := baggageclaimcmd.BaggageclaimCommand{
	//	Logger: flag.Lager{LogLevel: "debug"},
	//}
	//workerConfig := cmd.WorkerConfig{
	//	Name: "some-worker",
	//}
	tsaConfig := worker.TSAConfig{
		WorkerPrivateKey: `-----BEGIN PRIVATE KEY-----
		private key is in ~/Documents/key on cupcake workstation
		-----END PRIVATE KEY-----`,
	}
	//flag.
	//
	//s.workerCommand = &cmd.WorkerCommand{
	//	Worker:                      workerConfig,
	//	TSA:                         tsaConfig,
	//	Certs:                       cmd.Certs{},
	//	WorkDir:                     "",
	//	BindIP:                      flag.IP{},
	//	BindPort:                    0,
	//	DebugBindIP:                 flag.IP{},
	//	DebugBindPort:               0,
	//	HealthcheckBindIP:           flag.IP{},
	//	HealthcheckBindPort:         0,
	//	HealthCheckTimeout:          0,
	//	SweepInterval:               0,
	//	VolumeSweeperMaxInFlight:    0,
	//	ContainerSweeperMaxInFlight: 0,
	//	RebalanceInterval:           0,
	//	ConnectionDrainTimeout:      0,
	//	Garden:                      cmd.GardenBackend{},
	//	ExternalGardenURL:           flag.URL{},
	//	Baggageclaim:                baggageClaimCmd,
	//	ResourceTypes:               "",
	//	Logger:                      flag.Lager{LogLevel: "debug"},
	//}

	workerCmd := &cmd.WorkerCommand{}
	args, err := flags.ParseArgs(workerCmd, []string{})
	require.NoError(s.T(), err, "something went wrong")

	runner, err := s.workerCommand.Runner(args)
	if err != nil {
		panic(err)
	}

	<-ifrit.Invoke(sigmon.New(runner)).Wait()
	return
}

func (s *WorkerSuite) TearDownSuite() {
	fmt.Println("teardown suite")
}

func (s *WorkerSuite) SetupTest() {
	fmt.Println("setup test")
}

func (s *WorkerSuite) TearDownTest() {
	fmt.Println("teardown suite")
}

func TestSuite(t *testing.T) {
	suite.Run(t, &WorkerSuite{
		Assertions: require.New(t),
	})
}
