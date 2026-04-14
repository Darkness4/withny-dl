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

func TestFindStreamUUID(t *testing.T) {
	suuid, err := findStreamUUID(strings.NewReader(fixtureChannelHTML))

	require.NoError(t, err)
	require.Equal(
		t,
		"946b6d3b-05d1-481f-a053-180f592be1ad",
		suuid,
	)
}
