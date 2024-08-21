package withny_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/Darkness4/withny-dl/withny"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/stretchr/testify/require"
)

func TestPrepareFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	format := fmt.Sprintf("%s/{{ .Title }}.{{ .Ext }}", dir)
	fName, err := withny.PrepareFileAutoRename(format, api.MetaData{
		Stream: api.GetStreamsResponseElement{
			Title: "test",
		},
	}, withny.DefaultParams.Labels, "mp4")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s/test.mp4", dir), fName)

	require.NoError(t, os.WriteFile(fName, []byte("test"), 0o600))

	fName, err = withny.PrepareFileAutoRename(format, api.MetaData{
		Stream: api.GetStreamsResponseElement{
			Title: "test",
		},
	}, withny.DefaultParams.Labels, "mp4")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s/test.1.mp4", dir), fName)
}
