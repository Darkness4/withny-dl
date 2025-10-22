//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	_ "embed"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/stretchr/testify/require"
)

func TestSessionWebSocket(t *testing.T) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	require.NoError(t, err)
	hclient := &http.Client{
		Jar:     jar,
		Timeout: time.Minute,
	}
	client := api.NewClient(
		hclient,
		&secret.CredentialsFromEnv{},
		secret.NewFileCache("/tmp/withny-dl-test.json", "withny-dl-test-secret"),
		api.WithClearCredentialCacheOnFailureAfter(300),
	)
	scraper := api.Scraper{client}
	_, suuid, err := scraper.FetchCommentsGraphQLAndStreamUUID(context.Background(), "admin", "")
	require.NoError(t, err)
	ws := api.NewSessionWebSocket(client, suuid, "")

	t.Run("Watch", func(t *testing.T) {
		ctx := context.Background()
		err := client.Login(ctx)
		require.NoError(t, err)

		conn, err := ws.Dial(ctx)
		require.NoError(t, err)

		streamsCh := make(chan *api.GetStreamsResponseElement, 10)
		go ws.Watch(ctx, conn, streamsCh)

		for {
			select {
			case stream := <-streamsCh:
				t.Log(stream)
			case <-time.After(10 * time.Second):
				return
			}
		}
	})
}
