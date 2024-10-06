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

func TestWebSocket(t *testing.T) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	require.NoError(t, err)
	hclient := &http.Client{
		Jar:     jar,
		Timeout: time.Minute,
	}
	client := api.NewClient(hclient, &secret.UserPasswordFromEnv{}, secret.NewTmpCache())
	scraper := api.NewScraper(client)
	wsURL, suuid, err := scraper.FindGraphQLAndStreamUUID(context.Background(), "admin")
	require.NoError(t, err)
	ws := api.NewWebSocket(client, wsURL)

	t.Run("WatchComments", func(t *testing.T) {
		ctx := context.Background()
		err := client.Login(ctx)
		require.NoError(t, err)

		conn, err := ws.Dial(ctx)
		require.NoError(t, err)

		commentsCh := make(chan *api.Comment, 10)
		go ws.WatchComments(ctx, conn, suuid, commentsCh)

		for {
			select {
			case comment := <-commentsCh:
				t.Log(comment)
			case <-time.After(10 * time.Second):
				return
			}
		}
	})
}
