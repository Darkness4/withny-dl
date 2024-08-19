// withny-dl is a tool to download withny streams.
package main

import (
	"os"

	"github.com/Darkness4/withny-dl/cmd/clean"
	"github.com/Darkness4/withny-dl/cmd/concat"
	"github.com/Darkness4/withny-dl/cmd/download"
	"github.com/Darkness4/withny-dl/cmd/remux"
	"github.com/Darkness4/withny-dl/cmd/watch"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var version = "dev"

func init() {
	log.Logger = log.Logger.Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

var app = &cli.App{
	Name:    "withny-dl",
	Usage:   "withny download.",
	Version: version,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:       "debug",
			EnvVars:    []string{"DEBUG"},
			Value:      false,
			HasBeenSet: true,
			Action: func(_ *cli.Context, s bool) error {
				if s {
					log.Logger = log.Logger.Level(zerolog.DebugLevel)
					zerolog.SetGlobalLevel(zerolog.DebugLevel)
				}
				return nil
			},
		},
		&cli.BoolFlag{
			Name:       "trace",
			EnvVars:    []string{"TRACE"},
			Value:      false,
			HasBeenSet: true,
			Action: func(_ *cli.Context, s bool) error {
				if s {
					log.Logger = log.Logger.Level(zerolog.TraceLevel)
					zerolog.SetGlobalLevel(zerolog.TraceLevel)
				}
				return nil
			},
		},
		&cli.BoolFlag{
			Name:       "log-json",
			EnvVars:    []string{"LOG_JSON"},
			Value:      false,
			HasBeenSet: true,
			Action: func(_ *cli.Context, s bool) error {
				if !s {
					log.Logger = log.Logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
				}
				return nil
			},
		},
	},
	Commands: []*cli.Command{
		download.Command,
		watch.Command,
		remux.Command,
		concat.Command,
		clean.Command,
	},
}

func main() {
	log.Logger = log.Logger.With().Caller().Logger()
	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("application finished")
	}
}
