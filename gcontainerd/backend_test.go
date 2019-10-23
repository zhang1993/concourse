package gcontainerd_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions
}

func (suite *BackendSuite) SetupSuite() {

}

func (suite *BackendSuite) TearDownSuite() {
	fmt.Println("teardown suite")
}

func (suite *BackendSuite) SetupTest() {
	fmt.Println("setup test")
}

func (suite *BackendSuite) TearDownTest() {
	fmt.Println("teardown suite")
}

func TestSuite(t *testing.T) {
	suite.Run(t, &BackendSuite{
		Assertions: require.New(t),
	})
}

func (suite *BackendSuite) TestContainerCreation() {
	suite.Equal(4, 5, "4 is not the same as 5")
}
