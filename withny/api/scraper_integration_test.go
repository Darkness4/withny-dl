//go:build contract

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	_ "embed"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/stretchr/testify/require"
)

func TestScraper(t *testing.T) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	require.NoError(t, err)
	hclient := &http.Client{Jar: jar, Timeout: time.Minute}
	client := api.NewClient(
		hclient,
		&secret.CredentialsFromEnv{},
		secret.NewFileCache("/tmp/withny-dl-test.json", "withny-dl-test-secret"),
		api.WithClearCredentialCacheOnFailureAfter(5),
	)
	scraper := api.Scraper{client}

	t.Run("FetchStreamUUID", func(t *testing.T) {
		suuid, err := scraper.FetchStreamUUID(
			context.Background(),
			"admin",
			"",
		)
		require.NoError(t, err)
		fmt.Println("Stream UUID:", suuid)
		require.NotEmpty(t, suuid)
	})
}
