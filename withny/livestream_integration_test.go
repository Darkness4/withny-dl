//go:build integration

package withny_test

import (
	"testing"

	"github.com/Darkness4/withny-dl/withny"
	"github.com/stretchr/testify/suite"
)

type LiveStreamIntegrationTestSuite struct {
	suite.Suite
	impl *withny.LiveStream
}

// func (suite *LiveStreamIntegrationTestSuite) BeforeTest(suiteName, testName string) {
// 	jar, err := cookiejar.New(&cookiejar.Options{})
// 	if err != nil {
// 		panic(err)
// 	}
// 	suite.impl = withny.NewLiveStream(withny.NewClient(&http.Client{
// 		Jar: jar,
// 	}, withny.UserPasswordFromEnv{}), "8829230")
// }

func TestLiveStreamIntegrationTestSuite(t *testing.T) {
	suite.Run(t, &LiveStreamIntegrationTestSuite{})
}
