package withny

import (
	"bytes"
	"text/template"
	"time"

	"github.com/Darkness4/withny-dl/utils"
	"github.com/Darkness4/withny-dl/withny/api"
)

// FormatOutput formats the output file name.
func FormatOutput(
	outFormat string,
	meta api.Metadata,
	labels map[string]string,
	ext string,
) (string, error) {
	timeNow := time.Now()
	formatInfo := struct {
		ChannelID   string
		ChannelName string
		Date        string
		Time        string
		Title       string
		Ext         string
		MetaData    api.Metadata
		Labels      map[string]string
	}{
		Date:   timeNow.Format("2006-01-02"),
		Time:   timeNow.Format("150405"),
		Ext:    ext,
		Labels: labels,
	}

	tmpl, err := template.New("gotpl").Parse(outFormat)
	if err != nil {
		return "", err
	}

	formatInfo.ChannelID = utils.SanitizeFilename(meta.User.Username)
	formatInfo.ChannelName = utils.SanitizeFilename(meta.User.Name)
	formatInfo.Title = utils.SanitizeFilename(meta.Stream.Title)
	formatInfo.MetaData = meta

	var formatted bytes.Buffer
	if err = tmpl.Execute(&formatted, formatInfo); err != nil {
		return "", err
	}

	return formatted.String(), nil
}
