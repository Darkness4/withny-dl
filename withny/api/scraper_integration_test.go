//go:build contract

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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
		api.WithClearCredentialCacheOnFailureAfter(300),
	)
	scraper := api.Scraper{client}

	t.Run("FindGraphQLAndStreamUUID", func(t *testing.T) {
		out, suuid, err := scraper.FetchCommentsGraphQLAndStreamUUID(
			context.Background(),
			"admin",
			"",
		)
		require.NoError(t, err)
		require.NotEmpty(t, out)
		fmt.Println(out, suuid)
		_, err = url.Parse(out)
		require.NoError(t, err)
		require.NotEmpty(t, suuid)
	})
}
