//go:build integration

package withny_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/stretchr/testify/suite"
)

type ChannelWatcherIntegrationTestSuite struct {
	suite.Suite
	ctx    context.Context
	client *api.Client
	impl   *withny.ChannelWatcher
}

func (suite *ChannelWatcherIntegrationTestSuite) BeforeTest(suiteName, testName string) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}
	suite.client = api.NewClient(&http.Client{
		Jar: jar,
	}, secret.CredentialsFromEnv{}, secret.NewTmpCache())
	suite.ctx = context.Background()
	suite.impl = withny.NewChannelWatcher(suite.client, &withny.Params{
		PacketLossMax:          20,
		OutFormat:              "{{ .Date }} {{ .Title }} ({{ .ChannelName }}).{{ .Ext }}",
		WaitPollInterval:       10 * time.Second,
		Remux:                  true,
		Concat:                 true,
		KeepIntermediates:      true,
		ScanDirectory:          "",
		EligibleForCleaningAge: 48 * time.Hour,
		DeleteCorrupted:        true,
		ExtractAudio:           true,
	}, os.Getenv("WITHNY_STREAM_USERNAME"))
}

func (suite *ChannelWatcherIntegrationTestSuite) TestWatch() {
	// Act
	suite.impl.Watch(suite.ctx)
}

func (suite *ChannelWatcherIntegrationTestSuite) TestHasNewStream() {
	// Act
	res, err := suite.impl.HasNewStream(context.Background())

	// Assert
	suite.Require().NoError(err)
	suite.Require().Equal(true, res.HasNewStream)
	suite.Require().NotEmpty(res.Stream)
	suite.Require().NotEmpty(res.PlaybackURL)
}

func TestChannelWatcherIntegrationTestSuite(t *testing.T) {
	suite.Run(t, &ChannelWatcherIntegrationTestSuite{})
}
