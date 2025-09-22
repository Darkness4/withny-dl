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
)

func TestFindGraphQLEndpointAndStreamUUID(t *testing.T) {
	endpoint, suuid, err := findGraphQLEndpointAndStreamUUID(strings.NewReader(fixtureChannelHTML))

	require.NoError(t, err)
	require.Equal(
		t,
		"https://77fkxz2qsvclbkkbvzxjt2jley.appsync-api.ap-northeast-1.amazonaws.com/graphql",
		endpoint,
	)
	require.Equal(
		t,
		"4176f168-a4ed-49e3-8b55-13d50fbfb6f2",
		suuid,
	)
}
