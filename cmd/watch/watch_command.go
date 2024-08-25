// Package watch provides the watch command for watching multiple live withny streams.
package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	// Import the pprof package to enable profiling via HTTP.
	_ "net/http/pprof"
	// Import the godeltaprof package to enable continuous profiling via Pyroscope.
	_ "github.com/grafana/pyroscope-go/godeltaprof/http/pprof"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Darkness4/withny-dl/notify"
	"github.com/Darkness4/withny-dl/notify/notifier"
	"github.com/Darkness4/withny-dl/state"
	"github.com/Darkness4/withny-dl/telemetry"
	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/utils/try"
	"github.com/Darkness4/withny-dl/withny"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/Darkness4/withny-dl/withny/cleaner"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

// Hardcoded URL to check for new versions.
const versionCheckURL = "https://api.github.com/repos/Darkness4/withny-dl/releases/latest"

var (
	configPath             string
	pprofListenAddress     string
	enableTracesExporting  bool
	enableMetricsExporting bool
)

// Command is the command for watching multiple live withny streams.
var Command = &cli.Command{
	Name:  "watch",
	Usage: "Automatically download multiple Live withny streams.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Aliases:     []string{"c"},
			Required:    true,
			Usage:       `Config file path. (required)`,
			Destination: &configPath,
		},
		&cli.StringFlag{
			Name:        "pprof.listen-address",
			Value:       ":3000",
			Destination: &pprofListenAddress,
			Usage:       "The address to listen on for pprof.",
			EnvVars:     []string{"PPROF_LISTEN_ADDRESS"},
		},
		&cli.BoolFlag{
			Name:        "traces.export",
			Usage:       "Enable traces push. (To configure the exporter, set the OTEL_EXPORTER_OTLP_ENDPOINT environment variable, see https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/)",
			Value:       false,
			Destination: &enableTracesExporting,
			EnvVars:     []string{"OTEL_EXPORTER_OTLP_TRACES_ENABLED"},
		},
		&cli.BoolFlag{
			Name:        "metrics.export",
			Usage:       "Enable metrics push. (To configure the exporter, set the OTEL_EXPORTER_OTLP_ENDPOINT environment variable, see https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/). Note that a Prometheus path is already exposed at /metrics.",
			Value:       false,
			Destination: &enableMetricsExporting,
			EnvVars:     []string{"OTEL_EXPORTER_OTLP_METRICS_ENABLED"},
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

		// Setup telemetry
		prom, err := prometheus.New()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create prometheus exporter")
		}

		telOpts := []telemetry.Option{
			telemetry.WithMetricReader(prom),
		}

		if enableMetricsExporting {
			metricExporter, err := otlpmetricgrpc.New(ctx)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create OTEL metric exporter")
			}
			telOpts = append(telOpts, telemetry.WithMetricExporter(metricExporter))
		}

		if enableTracesExporting {
			traceExporter, err := otlptracegrpc.New(ctx)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create OTEL trace exporter")
			}
			telOpts = append(telOpts, telemetry.WithTraceExporter(traceExporter))
		}

		shutdown, err := telemetry.SetupOTELSDK(ctx,
			telOpts...,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to setup OTEL SDK")
		}
		defer func() {
			if err := shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Err(err).Msg("failed to shutdown OTEL SDK")
			}
		}()

		configChan := make(chan *Config)
		go ObserveConfig(ctx, configPath, configChan)

		go func() {
			http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				s := state.DefaultState.ReadState()
				b, err := json.MarshalIndent(s, "", "  ")
				if err != nil {
					log.Err(err).Msg("failed to marshal state")
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				_, err = w.Write(b)
				if err != nil {
					log.Err(err).Msg("failed to write state")
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			})
			http.Handle("/metrics", promhttp.Handler())
			log.Info().Str("listenAddress", pprofListenAddress).Msg("listening")
			if err := http.ListenAndServe(pprofListenAddress, nil); err != nil {
				log.Fatal().Err(err).Msg("fail to serve http")
			}
			log.Fatal().Msg("http server stopped")
		}()

		return ConfigReloader(ctx, configChan, func(ctx context.Context, config *Config) {
			handleConfig(ctx, cCtx.App.Version, config)
		})
	},
}

func handleConfig(ctx context.Context, version string, config *Config) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		log.Panic().Err(err).Msg("failed to initialize cookie jar")
	}

	params := withny.DefaultParams.Clone()
	config.DefaultParams.Override(params)

	hclient := &http.Client{
		Jar:     jar,
		Timeout: time.Minute,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithTracerProvider(noop.NewTracerProvider()),
		),
	}

	if config.CredentialsFile == "" {
		log.Fatal().Msg("no credentials file configured")
	}
	client := api.NewClient(hclient, secret.NewReader(config.CredentialsFile))

	go func() {
		if err := client.LoginLoop(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info().Msg("abort login")
				return
			}

			log.Fatal().Err(err).Msg("failed to login")
		}
	}()

	if config.Notifier.Enabled {
		notifier.Notifier = notify.NewFormatedNotifier(
			notify.NewShoutrrr(
				config.Notifier.URLs,
				notify.IncludeTitleInMessage(config.Notifier.IncludeTitleInMessage),
				notify.NoPriority(config.Notifier.NoPriority),
			),
			config.Notifier.NotificationFormats,
		)
		log.Info().Msg("using shoutrrr")
		if len(config.Notifier.URLs) == 0 {
			log.Warn().Msg("using shoutrrr but there is no URLs")
		}
	} else {
		log.Info().Msg("no notifier configured")
	}

	if err := notifier.NotifyConfigReloaded(ctx); err != nil {
		log.Err(err).Msg("notify failed")
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			if err := notifier.NotifyPanicked(ctx, err); err != nil {
				log.Err(err).Msg("notify failed")
			}
			os.Exit(1)
		}
	}()

	// Check new version
	go checkVersion(ctx, client.Client, version)

	var wg sync.WaitGroup
	wg.Add(len(config.Channels))
	for channel, overrideParams := range config.Channels {
		channelParams := params.Clone()
		overrideParams.Override(channelParams)

		// Scan for intermediates .ts used for concatenation
		if !channelParams.KeepIntermediates && channelParams.Concat &&
			channelParams.ScanDirectory != "" {
			wg.Add(1)
			go func(params *withny.Params) {
				defer wg.Done()
				cleaner.CleanPeriodically(
					ctx,
					params.ScanDirectory,
					time.Hour,
					cleaner.WithEligibleAge(params.EligibleForCleaningAge),
				)
			}(channelParams)
		}

		go func(channelID string, params *withny.Params) {
			defer wg.Done()
			log := log.With().Str("channelID", channelID).Logger()
			for {
				state.DefaultState.SetChannelState(
					channelID,
					state.DownloadStateIdle,
					state.WithLabels(params.Labels),
				)
				if err := notifier.NotifyIdle(ctx, channelID, params.Labels); err != nil {
					log.Err(err).Msg("notify failed")
				}

				meta, err := handleChannel(ctx, client, channelID, params)
				if errors.Is(err, context.Canceled) {
					log.Info().Msg("abort watching channel")
					if state.DefaultState.GetChannelState(
						channelID,
					) != state.DownloadStateIdle {
						state.DefaultState.SetChannelState(
							channelID,
							state.DownloadStateCanceled,
							state.WithLabels(params.Labels),
						)
						if err := notifier.NotifyCanceled(
							context.Background(),
							channelID,
							params.Labels,
						); err != nil {
							log.Err(err).Msg("notify failed")
						}
					}
					return
				} else if err != nil {
					log.Err(err).Msg("failed to download")
					state.DefaultState.SetChannelError(channelID, err)
					if err := notifier.NotifyError(
						context.Background(),
						channelID,
						params.Labels,
						err,
					); err != nil {
						log.Err(err).Msg("notify failed")
					}

					var getpburlErr api.GetPlaybackURLError
					if errors.As(err, &getpburlErr) {
						log.Error().Msg("failed to get playback URL, waiting for stream to end, backing off...")
						err := try.DoExponentialBackoff(60, 1*time.Minute, 2, 60*time.Minute, func() error {
							streams, err := client.GetStreams(ctx, channelID)
							if err != nil {
								if err := notifier.NotifyError(
									context.Background(),
									channelID,
									params.Labels,
									err,
								); err != nil {
									log.Err(err).Msg("notify failed")
								}
								return err
							}

							if len(streams) == 0 {
								return nil
							}
							_, err = client.GetStreamPlaybackURL(ctx, streams[0].UUID)
							if err != nil {
								if err := notifier.NotifyError(
									context.Background(),
									channelID,
									params.Labels,
									err,
								); err != nil {
									log.Err(err).Msg("notify failed")
								}
								return err
							}
							return nil
						})
						if err != nil {
							log.Err(err).Msg("failed to login and fetch playback URL")
						}
					}
				} else {
					state.DefaultState.SetChannelState(
						channelID,
						state.DownloadStateFinished,
						state.WithLabels(params.Labels),
					)
					if err := notifier.NotifyFinished(ctx, channelID, params.Labels, meta); err != nil {
						log.Err(err).Msg("notify failed")
					}
				}
				time.Sleep(time.Second)
			}
		}(channel, channelParams)

		// Spread out the channel start time to avoid hammering the server.
		time.Sleep(config.RateLimitAvoidance.PollingPacing)
	}

	wg.Wait()
}

func checkVersion(ctx context.Context, client *http.Client, version string) {
	if strings.Contains(version, "-") { // Version containing a hyphen is a development version.
		log.Warn().Str("version", version).Msg("development version, skipping version check")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionCheckURL, nil)
	if err != nil {
		log.Err(err).Msg("failed to create request")
		return
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to check version")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Str("status", resp.Status).Msg("failed to check version")
		return
	}

	var data struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Err(err).Msg("failed to decode version")
		return
	}

	if data.TagName != version {
		log.Warn().Str("latest", data.TagName).Str("current", version).Msg("new version available")
		if err := notifier.NotifyUpdateAvailable(ctx, data.TagName); err != nil {
			log.Err(err).Msg("notify failed")
		}
	}
}

func handleChannel(
	ctx context.Context,
	client *api.Client,
	channelID string,
	params *withny.Params,
) (api.MetaData, error) {
	downloader := withny.NewChannelWatcher(client, params, channelID)

	meta, err := downloader.Watch(ctx)
	if err != nil && err != io.EOF {
		log.Err(err).Msg("watcher has stopped")
		return api.MetaData{}, err
	}
	return meta, nil
}
