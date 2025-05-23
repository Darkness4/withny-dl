package watch

import (
	"context"
	"os"
	"time"

	"github.com/Darkness4/withny-dl/notify"
	"github.com/Darkness4/withny-dl/utils/channel"
	"github.com/Darkness4/withny-dl/withny"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Config is the configuration for the watch command.
type Config struct {
	Notifier                           NotifierConfig                   `yaml:"notifier,omitempty"`
	RateLimitAvoidance                 RateLimitAvoidance               `yaml:"rateLimitAvoidance,omitempty"`
	CredentialsFile                    string                           `yaml:"credentialsFile,omitempty"`
	CachedCredentialsFile              string                           `yaml:"cachedCredentialsFile,omitempty"`
	ClearCredentialCacheOnFailureAfter int                              `yaml:"clearCredentialCacheOnFailureAfter,omitempty"`
	DefaultParams                      withny.OptionalParams            `yaml:"defaultParams,omitempty"`
	Channels                           map[string]withny.OptionalParams `yaml:"channels,omitempty"`
}

// NotifierConfig is the configuration for the notifier.
type NotifierConfig struct {
	Enabled                    bool     `yaml:"enabled,omitempty"`
	IncludeTitleInMessage      bool     `yaml:"includeTitleInMessage,omitempty"`
	NoPriority                 bool     `yaml:"noPriority,omitempty"`
	URLs                       []string `yaml:"urls,omitempty"`
	notify.NotificationFormats `         yaml:"notificationFormats,omitempty"`
}

// RateLimitAvoidance is the configuration for the rate limit avoidance.
type RateLimitAvoidance struct {
	PollingPacing time.Duration `yaml:"pollingPacing,omitempty"`
}

func applyDefaults(config *Config) {
	if config.RateLimitAvoidance.PollingPacing == 0 {
		config.RateLimitAvoidance.PollingPacing = 500 * time.Millisecond
	}
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}
	applyDefaults(config)
	return config, err
}

// ObserveConfig watches the config file for changes and sends the new config to the configChan.
func ObserveConfig(ctx context.Context, filename string, configChan chan<- *Config) {
	var lastModTime time.Time

	// Initial load
	func() {
		stat, err := os.Stat(filename)
		if err != nil {
			log.Error().Str("file", filename).Err(err).Msg("failed to stat file")
			return
		}
		lastModTime = stat.ModTime()

		log.Info().Msg("initial config detected")
		config, err := loadConfig(filename)
		if err != nil {
			log.Error().Str("file", filename).Err(err).Msg("failed to load config")
			return
		}

		configChan <- config
	}()

	// Use ticker as fallback in case fsnotify fails
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Panic().Err(err).Msg("failed to watch config")
	}
	defer watcher.Close()

	if err = watcher.Add(filename); err != nil {
		log.Panic().Err(err).Msg("failed to add config to config reloader")
	}

	debouncedEvents := channel.Debounce(watcher.Events, time.Second)

	for {
		select {
		case <-ctx.Done():
			// The parent context was canceled, exit the loop
			log.Err(ctx.Err()).Msg("watcher context canceled")
			return
		case <-ticker.C:
			lastModTime, err = loadConfigOnModification(ctx, filename, configChan, lastModTime)
			if err != nil {
				continue
			}

		case _, ok := <-debouncedEvents:
			if !ok {
				log.Error().Msg("watcher channel closed")
				return
			}
			lastModTime, err = loadConfigOnModification(ctx, filename, configChan, lastModTime)
			if err != nil {
				continue
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				log.Error().Msg("watcher error channel closed")
				return
			}
			log.Error().Str("file", filename).Err(err).Msg("config reloader thrown an error")
		}
	}
}

func loadConfigOnModification(
	ctx context.Context,
	filename string,
	configChan chan<- *Config,
	lastModTime time.Time,
) (time.Time, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		log.Error().Str("file", filename).Err(err).Msg("failed to stat file")
		return lastModTime, err
	}

	if !stat.ModTime().Equal(lastModTime) {
		lastModTime = stat.ModTime()
		log.Info().Msg("new config detected")

		config, err := loadConfig(filename)
		if err != nil {
			log.Error().Str("file", filename).Err(err).Msg("failed to load config")
			return lastModTime, err
		}
		select {
		case configChan <- config:
			// Config sent successfully
		case <-ctx.Done():
			// The parent context was canceled, exit the loop
			log.Err(ctx.Err()).
				Msg("config reloader context canceled while the config was being sent")
			return lastModTime, ctx.Err()
		}
	}
	return lastModTime, nil
}

// ConfigReloader reloads the config when a new one is detected.
func ConfigReloader(
	ctx context.Context,
	configChan <-chan *Config,
	handleConfig func(ctx context.Context, config *Config),
) error {
	var configContext context.Context
	var configCancel context.CancelFunc
	// Channel used to assure only one handleConfig can be launched
	doneChan := make(chan struct{})

	for {
		select {
		case newConfig := <-configChan:
			if configContext != nil && configCancel != nil {
				configCancel()
				select {
				case <-doneChan:
					log.Info().Msg("loading new config")
				case <-time.After(30 * time.Second):
					log.Fatal().Msg("couldn't load a new config because of a deadlock")
				}
			}
			configContext, configCancel = context.WithCancel(ctx)
			go func() {
				log.Info().Msg("loaded new config")
				handleConfig(configContext, newConfig)
				doneChan <- struct{}{}
			}()
		case <-ctx.Done():
			if configContext != nil && configCancel != nil {
				configCancel()
				configContext = nil
			}

			// This assure that the `handleConfig` ends gracefully
			select {
			case <-doneChan:
				log.Info().Msg("config reloader graceful exit")
			case <-time.After(30 * time.Second):
				log.Fatal().Msg("config reloader force fatal exit")
			}

			// The context was canceled, exit the loop
			return ctx.Err()
		}
	}
}
