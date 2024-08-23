// Package download provide a command for downloading a live withny stream.
package download

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/utils/try"
	"github.com/Darkness4/withny-dl/withny"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var (
	downloadParams = withny.Params{}
	maxTries       int
	loop           bool

	credentialFile    string
	credentialsStatic secret.Static
)

// Command is the command for downloading a live withny stream.
var Command = &cli.Command{
	Name:      "download",
	Usage:     "Download a withny live stream.",
	ArgsUsage: "channelID",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:        "quality.min-height",
			Category:    "Streaming:",
			Usage:       `Minimum inclusive height of the stream.`,
			Destination: &downloadParams.QualityConstraint.MinHeight,
		},
		&cli.Int64Flag{
			Name:        "quality.max-height",
			Category:    "Streaming:",
			Usage:       `Maximum inclusive height of the stream.`,
			Destination: &downloadParams.QualityConstraint.MaxHeight,
		},
		&cli.Int64Flag{
			Name:        "quality.min-width",
			Category:    "Streaming:",
			Usage:       `Minimum inclusive width of the stream.`,
			Destination: &downloadParams.QualityConstraint.MinWidth,
		},
		&cli.Int64Flag{
			Name:        "quality.max-width",
			Category:    "Streaming:",
			Usage:       `Maximum inclusive width of the stream.`,
			Destination: &downloadParams.QualityConstraint.MaxWidth,
		},
		&cli.Float64Flag{
			Name:        "quality.min-framerate",
			Category:    "Streaming:",
			Usage:       `Minimum inclusive framerate of the stream.`,
			Destination: &downloadParams.QualityConstraint.MinFrameRate,
		},
		&cli.Float64Flag{
			Name:        "quality.max-framerate",
			Category:    "Streaming:",
			Usage:       `Maximum inclusive framerate of the stream.`,
			Destination: &downloadParams.QualityConstraint.MaxFrameRate,
		},
		&cli.Int64Flag{
			Name:        "quality.min-bandwidth",
			Category:    "Streaming:",
			Usage:       `Minimum inclusive bandwidth of the stream.`,
			Destination: &downloadParams.QualityConstraint.MinBandwidth,
		},
		&cli.Int64Flag{
			Name:        "quality.max-bandwidth",
			Category:    "Streaming:",
			Usage:       `Maximum inclusive bandwidth of the stream.`,
			Destination: &downloadParams.QualityConstraint.MaxBandwidth,
		},
		&cli.BoolFlag{
			Name:        "quality.audio-only",
			Category:    "Streaming:",
			Usage:       "Only download audio streams.",
			Destination: &downloadParams.QualityConstraint.AudioOnly,
		},
		&cli.StringFlag{
			Name:     "format",
			Value:    "{{ .Date }} {{ .Title }} ({{ .ChannelName }}).{{ .Ext }}",
			Category: "Post-Processing:",
			Usage: `Golang templating format. Available fields: ChannelID, ChannelName, Date, Time, Title, Ext, Labels.Key.
Available format options:
  ChannelID: ID of the broadcast
  ChannelName: broadcaster's profile name
  Date: local date YYYY-MM-DD
  Time: local time HHMMSS
  Ext: file extension
  Title: title of the live broadcast
  Labels.Key: custom labels
`,
			Destination: &downloadParams.OutFormat,
		},
		&cli.BoolFlag{
			Name:        "write-chat",
			Value:       false,
			Category:    "Streaming:",
			Usage:       "Save live chat into a json file.",
			Destination: &downloadParams.WriteChat,
		},
		&cli.BoolFlag{
			Name:        "write-metadata-json",
			Value:       false,
			Category:    "Streaming:",
			Usage:       "Dump output stream MetaData into a json file.",
			Destination: &downloadParams.WriteMetaDataJSON,
		},
		&cli.BoolFlag{
			Name:        "write-thumbnail",
			Value:       false,
			Category:    "Streaming:",
			Usage:       "Download thumbnail into a file.",
			Destination: &downloadParams.WriteThumbnail,
		},
		&cli.IntFlag{
			Name:        "max-packet-loss",
			Value:       20,
			Category:    "Post-Processing:",
			Usage:       "Allow a maximum of packet loss before aborting stream download.",
			Destination: &downloadParams.PacketLossMax,
		},
		&cli.BoolFlag{
			Name:       "no-remux",
			Value:      false,
			HasBeenSet: true,
			Category:   "Post-Processing:",
			Usage:      "Do not remux recordings into mp4/m4a after it is finished.",
			Action: func(_ *cli.Context, b bool) error {
				downloadParams.Remux = !b
				return nil
			},
		},
		&cli.StringFlag{
			Name:        "remux-format",
			Value:       "mp4",
			Category:    "Post-Processing:",
			Usage:       "Remux format of the video.",
			Destination: &downloadParams.RemuxFormat,
		},
		&cli.BoolFlag{
			Name:        "concat",
			Value:       false,
			Category:    "Post-Processing:",
			Usage:       "Concatenate and remux with previous recordings after it is finished. ",
			Destination: &downloadParams.Concat,
		},
		&cli.BoolFlag{
			Name:        "keep-intermediates",
			Value:       false,
			Category:    "Post-Processing:",
			Usage:       "Keep the raw .ts recordings after it has been remuxed.",
			Aliases:     []string{"k"},
			Destination: &downloadParams.KeepIntermediates,
		},
		&cli.StringFlag{
			Name:        "scan-directory",
			Value:       "",
			Category:    "Cleaning Routine:",
			Usage:       "Directory to be scanned for .ts files to be deleted after concatenation.",
			Destination: &downloadParams.ScanDirectory,
		},
		&cli.DurationFlag{
			Name:        "eligible-for-cleaning-age",
			Value:       48 * time.Hour,
			Category:    "Cleaning Routine:",
			Usage:       "Minimum age of .combined files to be eligible for cleaning.",
			Aliases:     []string{"cleaning-age"},
			Destination: &downloadParams.EligibleForCleaningAge,
		},
		&cli.BoolFlag{
			Name:       "no-delete-corrupted",
			Value:      false,
			HasBeenSet: true,
			Category:   "Post-Processing:",
			Usage:      "Delete corrupted .ts recordings.",
			Action: func(_ *cli.Context, b bool) error {
				downloadParams.DeleteCorrupted = !b
				return nil
			},
		},
		&cli.BoolFlag{
			Name:        "extract-audio",
			Value:       false,
			Category:    "Post-Processing:",
			Usage:       "Generate an audio-only copy of the stream.",
			Aliases:     []string{"x"},
			Destination: &downloadParams.ExtractAudio,
		},
		&cli.PathFlag{
			Name:        "credentials-file",
			Usage:       "Path to a credentials file. Format is YAML and must contain 'username' and 'password' or 'access-token' and 'refresh-token'.",
			Category:    "Streaming:",
			Destination: &credentialFile,
		},
		&cli.StringFlag{
			Name:        "credentials.username",
			Usage:       "Username/email for withny login",
			Category:    "Streaming:",
			Aliases:     []string{"credentials.email"},
			Destination: &credentialsStatic.Username,
		},
		&cli.StringFlag{
			Name:        "credentials.password",
			Usage:       "Password for withny login",
			Category:    "Streaming:",
			Destination: &credentialsStatic.Password,
		},
		&cli.StringFlag{
			Name:        "credentials.access-token",
			Usage:       "Access token for withny login. You should also provide a refresh token.",
			Category:    "Streaming:",
			Destination: &credentialsStatic.Token,
		},
		&cli.StringFlag{
			Name:        "credentials.refresh-token",
			Usage:       "Refresh token for withny login.",
			Category:    "Streaming:",
			Destination: &credentialsStatic.RefreshToken,
		},
		&cli.BoolFlag{
			Name:       "no-wait",
			Value:      false,
			HasBeenSet: true,
			Category:   "Polling:",
			Usage:      "Don't wait until the broadcast goes live, then start recording.",
			Action: func(_ *cli.Context, b bool) error {
				downloadParams.WaitForLive = !b
				return nil
			},
		},
		&cli.DurationFlag{
			Name:        "poll-interval",
			Value:       5 * time.Second,
			Category:    "Polling:",
			Usage:       "How many seconds between checks to see if broadcast is live.",
			Destination: &downloadParams.WaitPollInterval,
		},
		&cli.IntFlag{
			Name:        "max-tries",
			Value:       10,
			Category:    "Polling:",
			Usage:       "On failure, keep retrying (cancellation and end of stream will still force abort).",
			Destination: &maxTries,
		},
		&cli.BoolFlag{
			Name:        "loop",
			Value:       false,
			Category:    "Polling:",
			Usage:       "Continue to download streams indefinitely.",
			Destination: &loop,
		},
	},
	Action: func(cCtx *cli.Context) error {
		ctx, cancel := context.WithCancel(cCtx.Context)

		// Trap cleanup
		cleanChan := make(chan os.Signal, 1)
		signal.Notify(cleanChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-cleanChan
			cancel()
		}()

		channelID := cCtx.Args().Get(0)
		if channelID == "" {
			log.Error().Msg("channel ID is empty")
			return errors.New("missing channel")
		}

		jar, err := cookiejar.New(&cookiejar.Options{})
		if err != nil {
			log.Panic().Err(err).Msg("failed to create cookie jar")
		}
		hclient := &http.Client{Jar: jar, Timeout: time.Minute}

		var reader api.CredentialsReader
		if credentialsStatic.Username != "" || credentialsStatic.Token != "" {
			reader = &credentialsStatic
		}
		if credentialFile != "" {
			reader = secret.NewReader(credentialFile)
		}
		client := api.NewClient(hclient, reader)

		if err := client.Login(ctx); err != nil {
			log.Err(err).
				Msg("failed to login to withny")
			return err
		}

		downloader := withny.NewChannelWatcher(client, &downloadParams, channelID)
		log.Info().Any("params", downloadParams).Msg("running")

		if loop {
			for {
				_, err := downloader.Watch(ctx)
				if errors.Is(err, context.Canceled) {
					log.Info().Str("channelID", channelID).Msg("abort watching channel")
					break
				}
				if err != nil {
					log.Err(err).Msg("failed to download")
				}
				time.Sleep(time.Second)
			}
			return nil
		}

		return try.DoExponentialBackoff(maxTries, time.Second, 2, time.Minute, func() error {
			_, err := downloader.Watch(ctx)
			if err == io.EOF || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		})
	},
}
