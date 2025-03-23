//go:build contract

package hls_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/Darkness4/withny-dl/hls"
	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func init() {
	log.Logger = log.Logger.With().Caller().Logger()
	_ = godotenv.Load(".env")
	_ = godotenv.Load(".env.test")
}

func findAnyLiveStream(t *testing.T, client *api.Client) (username string) {
	streams, err := client.GetStreams(context.Background(), "")
	require.NoError(t, err)

	if len(streams) == 0 {
		t.Skip("No live streams found")
	}

	// Find a live stream that is not restricted
	for _, stream := range streams {
		if stream.Price.String() == "0" {
			return stream.Cast.AgencySecret.ChannelName
		}
	}

	return streams[0].Cast.AgencySecret.ChannelName
}

type DownloaderIntegrationTestSuite struct {
	suite.Suite
	ctx       context.Context
	ctxCancel context.CancelFunc
	client    *api.Client
	impl      *hls.Downloader
}

func (suite *DownloaderIntegrationTestSuite) fetchPlaylist(
	client *api.Client,
) api.Playlist {
	// Act
	streams, err := client.GetStreams(context.Background(), findAnyLiveStream(suite.T(), client))
	suite.Require().NoError(err)
	suite.Require().Greater(len(streams), 0)

	playbackURL, err := client.GetStreamPlaybackURL(context.Background(), streams[0].UUID)
	suite.Require().NoError(err)

	playlists, err := client.GetPlaylists(context.Background(), playbackURL, 0)

	playlist, ok := api.GetBestPlaylist(playlists)
	if !ok {
		panic("no playlist found")
	}
	log.Info().Any("playlist", playlist).Msg("playlist found")

	return playlist
}

func (suite *DownloaderIntegrationTestSuite) BeforeTest(suiteName, testName string) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}
	hclient := &http.Client{
		Jar: jar,
	}
	suite.ctx, suite.ctxCancel = context.WithCancel(context.Background())

	credReader := &secret.CredentialsFromEnv{}

	// Check livestream
	suite.client = api.NewClient(
		hclient,
		credReader,
		secret.NewFileCache("/tmp/withny-dl-test.json", "withny-dl-test-secret"),
		api.WithClearCredentialCacheOnFailureAfter(300),
	)
	err = suite.client.Login(context.Background())
	suite.Require().NoError(err)

	// Fetch playlist
	playlist := suite.fetchPlaylist(suite.client)

	// Prepare implementation
	suite.impl = hls.NewDownloader(
		suite.client,
		playlist.URL,
		hls.WithPacketLossMax(8),
		hls.WithLogger(&log.Logger),
	)
}

func (suite *DownloaderIntegrationTestSuite) TestGetFragmentURLs() {
	urls, err := suite.impl.GetFragmentURLs(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(urls)
	fmt.Println(urls)
}

func (suite *DownloaderIntegrationTestSuite) TestRead() {
	ctx, cancel := context.WithCancel(suite.ctx)
	f, err := os.Create("output.ts")
	if err != nil {
		suite.Require().NoError(err)
		cancel()
		return
	}
	defer f.Close()

	errChan := make(chan error, 1)

	go func() {
		err := suite.impl.Read(ctx, f)
		if err != nil {
			errChan <- err
		}
	}()

	time.Sleep(10 * time.Second)
	cancel()

	for {
		select {
		case err := <-errChan:
			suite.Require().Error(err, context.Canceled.Error())
			return
		}
	}
}

func TestDownloaderIntegrationTestSuite(t *testing.T) {
	suite.Run(t, &DownloaderIntegrationTestSuite{})
}
