package api

import (
	"strings"
	"testing"

	_ "embed"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed fixtures/channel.html
	fixtureChannelHTML string

	//go:embed fixtures/main-app.js
	fixtureMainAppJS string
)

func TestFindMainAppAndStreamUUID(t *testing.T) {
	path, suuid, err := findMainAppAndStreamUUID(strings.NewReader(fixtureChannelHTML))

	require.NoError(t, err)
	require.Equal(t, "/_next/static/chunks/main-app-f7179bce2d74082f.js", path)
	require.Equal(t, "5134338f-976b-4fc1-8a2e-91da9d6a7a92", suuid)
}

func TestFindGraphQLEndpoint(t *testing.T) {
	endpoint, err := findGraphQLEndpoint(strings.NewReader(fixtureMainAppJS))

	require.NoError(t, err)
	require.Equal(
		t,
		"https://77fkxz2qsvclbkkbvzxjt2jley.appsync-api.ap-northeast-1.amazonaws.com/graphql",
		endpoint,
	)
}
