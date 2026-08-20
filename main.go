// withny-dl is a tool to download withny streams.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/Darkness4/withny-dl/cmd/clean"
	"github.com/Darkness4/withny-dl/cmd/concat"
	"github.com/Darkness4/withny-dl/cmd/remux"
	"github.com/Darkness4/withny-dl/cmd/watch"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

var version = "dev"

func init() {
	log.Logger = log.Logger.Level(zerolog.InfoLevel).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

var debugLevel bool
var traceLevel bool
var jsonLog bool

var rootCmd = &cli.Command{
	Name:    "withny-dl",
	Version: version,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "debug",
			Sources:     cli.EnvVars("DEBUG"),
			Value:       false,
			Destination: &debugLevel,
		},
		&cli.BoolFlag{
			Name:        "trace",
			Sources:     cli.EnvVars("TRACE"),
			Value:       false,
			Destination: &traceLevel,
		},
		&cli.BoolFlag{
			Name:        "log-json",
			Sources:     cli.EnvVars("LOG_JSON"),
			Value:       false,
			Destination: &jsonLog,
		},
	},
	Commands: []*cli.Command{
		watch.Command,
		remux.Command,
		concat.Command,
		clean.Command,
	},
	Before: func(ctx context.Context, _ *cli.Command) (context.Context, error) {
		if debugLevel {
			log.Logger = log.Logger.Level(zerolog.DebugLevel)
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
		if traceLevel {
			log.Logger = log.Logger.Level(zerolog.TraceLevel)
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
		}
		if !jsonLog {
			log.Logger = log.Logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		}
		return ctx, nil
	},
}

func main() {
	ctx := context.Background()
	if err := rootCmd.Run(ctx, os.Args); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal().Err(err).Msg("application finished")
	}
}
