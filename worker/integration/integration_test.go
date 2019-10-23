package integration_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type WorkerSuite struct {
	suite.Suite
	*require.Assertions
}

func (s *WorkerSuite) SetupSuite() {
	workerIP := os.Getenv("WORKER_HOST")
	s.Require().NotNil(workerIP, "must specify worker host")
	gardenPort := os.Getenv("WORKER_GARDEN_PORT")
	s.Require().NotNil(gardenPort, "must specify worker garden port")
	baggageClaimPort := os.Getenv("WORKER_BC_PORT")
	s.Require().NotNil(baggageClaimPort, "must specify worker garden port")

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
