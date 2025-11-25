// Package watch provides the watch command for watching multiple live withny streams.
package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
	"github.com/Darkness4/withny-dl/withny"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/Darkness4/withny-dl/withny/cleaner"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

const (
	_shutdownPeriod      = 10 * time.Second
	_shutdownHardPeriod  = 3 * time.Second
	_readinessDrainDelay = 5 * time.Second
)

// Hardcoded URL to check for new versions.
const versionCheckURL = "https://api.github.com/repos/Darkness4/withny-dl/releases/latest"

var (
	configPath             string
	pprofListenAddress     string
	encryptionKey          string
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
			Name:        "secret.encryptionKey",
			Value:       "WITHNY_DL_ENCRYPTION_KEY",
			Destination: &encryptionKey,
			Usage:       "An encryption secret to encrypt the cached refresh token.",
			EnvVars:     []string{"WITHNY_ENCRYPTION_KEY"},
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
		ctx, stop := context.WithCancelCause(cCtx.Context)

		// Trap cleanup
		cleanChan := make(chan os.Signal, 1)
		signal.Notify(cleanChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-cleanChan
			log.Warn().Stringer("signal", sig).Msg("Received signal, shutting down")
			stop(fmt.Errorf("signal received: %s", sig))
		}()

		// Setup telemetry
		prom, err := prometheus.New()
		if err != nil {
			return fmt.Errorf("failed to create prometheus exporter: %w", err)
		}

		telOpts := []telemetry.Option{
			telemetry.WithMetricReader(prom),
		}

		if enableMetricsExporting {
			metricExporter, err := otlpmetricgrpc.New(ctx)
			if err != nil {
				return fmt.Errorf("failed to create OTEL metric exporter: %w", err)
			}
			telOpts = append(telOpts, telemetry.WithMetricExporter(metricExporter))
		}

		if enableTracesExporting {
			traceExporter, err := otlptracegrpc.New(ctx)
			if err != nil {
				return fmt.Errorf("failed to create OTEL trace exporter: %w", err)
			}
			telOpts = append(telOpts, telemetry.WithTraceExporter(traceExporter))
		}

		shutdown, err := telemetry.SetupOTELSDK(ctx,
			telOpts...,
		)
		if err != nil {
			return fmt.Errorf("failed to setup OTEL SDK: %w", err)
		}
		defer func() {
			if err := shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Err(err).Msg("failed to shutdown OTEL SDK")
			}
		}()

		configChan := make(chan *Config)
		go ObserveConfig(ctx, configPath, configChan)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
			s := state.DefaultState.ReadState()
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			if err := enc.Encode(s); err != nil {
				log.Err(err).Msg("failed to write state")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, "OK")
		})
		mux.Handle("GET /metrics", promhttp.Handler())
		ongoingCtx, stopOngoingGracefully := context.WithCancel(
			log.Logger.WithContext(context.Background()),
		)
		srv := &http.Server{
			Addr:         pprofListenAddress,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
			BaseContext:  func(_ net.Listener) context.Context { return ongoingCtx },
		}
		go func() {
			log.Info().Str("listenAddress", pprofListenAddress).Msg("listening")
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}()

		cfgErr := ConfigReloader(ctx, configChan, func(ctx context.Context, config *Config) error {
			return handleConfig(ctx, cCtx.App.Version, config)
		})
		if cfgErr != nil {
			log.Err(cfgErr).Msg("config reloader stopped")
		}

		// ---GRACEFUL SHUTDOWN---
		stop(cfgErr)
		signal.Stop(cleanChan)
		log.Info().
			Stringer("delay", _readinessDrainDelay).
			Msg("Received shutdown signal, gracefully shutting down HTTP server.")

		// Give time for readiness check to propagate
		time.Sleep(_readinessDrainDelay)
		log.Info().Msg(
			"Readiness check propagated, now waiting for ongoing requests to finish.",
		)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), _shutdownPeriod)
		defer cancel()
		err = srv.Shutdown(shutdownCtx)
		stopOngoingGracefully()
		if err != nil {
			log.Info().Msg(
				"Failed to wait for ongoing requests to finish, waiting for forced cancellation.",
			)
			time.Sleep(_shutdownHardPeriod)
		}
		log.Info().Msg("Server shut down gracefully.")

		return cfgErr
	},
}

func handleConfig(ctx context.Context, version string, config *Config) error {
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
		return errors.New("no credentials file configured")
	}
	if config.CachedCredentialsFile == "" {
		config.CachedCredentialsFile = "withny-dl.json"
	}
	client := api.NewClient(
		hclient,
		secret.NewReader(config.CredentialsFile),
		secret.NewFileCache(config.CachedCredentialsFile, encryptionKey),
		api.WithClearCredentialCacheOnFailureAfter(config.ClearCredentialCacheOnFailureAfter),
		api.WithUserAgent(config.UserAgent),
		api.WithLoginRetryDelay(config.LoginRetryDelay),
	)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	go func() {
		if err := client.LoginLoop(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info().Msg("abort login")
				return
			}

			cancel(fmt.Errorf("failed to login: %w", err))
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
			log.Err(fmt.Errorf("panic: %v", err)).Stack().Msg("program panicked")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			if err := notifier.NotifyPanicked(ctx, err); err != nil {
				log.Err(err).Msg("notify failed")
			}
			os.Exit(2)
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
			withny.NewChannelWatcher(&api.Scraper{Client: client}, params, channelID).Watch(ctx)

			select {
			case <-ctx.Done():
				log.Err(ctx.Err()).AnErr("cause", ctx.Err()).Msg("channel watcher stopped")
				return
			default:
				log.Panic().Msg("channel watcher stopped before parent context is canceled")
			}
		}(channel, channelParams)

		// Spread out the channel start time to avoid hammering the server.
		time.Sleep(config.RateLimitAvoidance.PollingPacing)
	}

	wg.Wait()

	return nil
}

func checkVersion(ctx context.Context, client *http.Client, version string) {
	if strings.Contains(version, "dev") ||
		strings.Contains(version, "-") { // Version containing a hyphen is a development version.
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
