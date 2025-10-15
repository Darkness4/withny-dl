// Package withny provides a way to watch a withny channel.
package withny

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Darkness4/withny-dl/notify/notifier"
	"github.com/Darkness4/withny-dl/state"
	"github.com/Darkness4/withny-dl/telemetry/metrics"
	"github.com/Darkness4/withny-dl/utils/try"
	"github.com/Darkness4/withny-dl/video/concat"
	"github.com/Darkness4/withny-dl/video/probe"
	"github.com/Darkness4/withny-dl/video/remux"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName    = "withny"
	commentBufMax = 100
)

var (
	// ErrLiveStreamNotOnline is returned when the live stream is not online.
	ErrLiveStreamNotOnline = errors.New("live stream is not online")
)

// ChannelWatcher is responsible to watch a withny channel.
type ChannelWatcher struct {
	*api.Client
	params *Params
	// filterChannelID is like a channelID, but an empty one will select all channels.
	filterChannelID string
	// processingStreams is a set of streamsIDs that are currently being processed.
	processingStreams     map[string]struct{}
	processingStreamsLock sync.Mutex
}

// NewChannelWatcher creates a new withny channel watcher.
func NewChannelWatcher(client *api.Client, params *Params, channelID string) *ChannelWatcher {
	if client == nil {
		log.Panic().Msg("client is nil")
	}
	return &ChannelWatcher{
		Client:            client,
		params:            params,
		filterChannelID:   channelID,
		processingStreams: make(map[string]struct{}),
	}
}

// Watch watches the channel for any new live stream.
func (w *ChannelWatcher) Watch(ctx context.Context) {
	log := log.With().Str("filterChannelID", w.filterChannelID).Logger()
	log.Info().Any("params", w.params).Msg("watching channel")
	ctx = log.WithContext(ctx)

	for {
		// Only handle IDLE state for a channelID not empty.
		// This is because an empty channelID means multiple channels are being watched.
		// Therefore, it is impossible to predict the true channelID that will be used.
		if w.filterChannelID != "" {
			state.DefaultState.SetChannelState(
				w.filterChannelID,
				state.DownloadStateIdle,
				state.WithLabels(w.params.Labels),
			)
			if err := notifier.NotifyIdle(ctx, w.filterChannelID, w.params.Labels); err != nil {
				log.Err(err).Msg("notify failed")
			}
		}

		res, err := w.HasNewStream(ctx)
		if err != nil {
			log.Err(err).Msg("failed to check if online")
		}

		if !res.HasNewStream {
			res = func() HasNewStreamResponse {
				ticker := time.NewTicker(w.params.WaitPollInterval)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						log.Err(ctx.Err()).Msg("channel watcher context done")
						return HasNewStreamResponse{}
					case <-ticker.C:
						res, err := w.HasNewStream(ctx)
						if err != nil {
							log.Err(err).Msg("failed to check if online")
							if errors.Is(err, context.Canceled) {
								return HasNewStreamResponse{}
							}
						} else if res.HasNewStream {
							return res
						}
					}
				}
			}()

			if !res.HasNewStream {
				// Context has been canceled.
				log.Warn().Msg("channel watcher context canceled, waiting for processing to finish")
				w.waitProcessingOrFatal(300 * time.Second)
				log.Warn().Msg("processing finished")
				return
			}
		}

		w.processingStreamsLock.Lock()
		w.processingStreams[res.Stream.UUID] = struct{}{}
		w.processingStreamsLock.Unlock()

		go func() {
			defer func() {
				w.processingStreamsLock.Lock()
				delete(w.processingStreams, res.Stream.UUID)
				w.processingStreamsLock.Unlock()
			}()
			log := log.With().Str("channelID", res.User.Username).Logger()
			ctx = log.WithContext(ctx)

			err := w.Process(ctx, api.MetaData{
				User:   res.User,
				Stream: res.Stream,
			}, res.PlaybackURL)

			if err != nil {
				if errors.Is(err, context.Canceled) {
					state.DefaultState.SetChannelState(
						res.User.Username,
						state.DownloadStateCanceled,
						state.WithLabels(w.params.Labels),
					)
					if err := notifier.NotifyCanceled(
						context.Background(),
						res.User.Username,
						w.params.Labels,
					); err != nil {
						log.Err(err).Msg("notify failed")
					}
				} else {
					state.DefaultState.SetChannelError(res.User.Username, err)
					if err := notifier.NotifyError(
						context.Background(),
						res.User.Username,
						w.params.Labels,
						err,
					); err != nil {
						log.Err(err).Msg("notify failed")
					}
				}
			} else {
				state.DefaultState.SetChannelState(
					res.User.Username,
					state.DownloadStateFinished,
					state.WithLabels(w.params.Labels),
				)
				if err := notifier.NotifyFinished(ctx, res.User.Username, w.params.Labels, api.MetaData{
					User:   res.User,
					Stream: res.Stream,
				}); err != nil {
					log.Err(err).Msg("notify failed")
				}
			}
		}()
	}
}

// waitProcessingOrFatal waits for the all the processes to finish.
func (w *ChannelWatcher) waitProcessingOrFatal(timeout time.Duration) {
	// Periodically check if all the processes are done.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ticker.C:
			w.processingStreamsLock.Lock()
			if len(w.processingStreams) == 0 {
				w.processingStreamsLock.Unlock()
				return
			}
			w.processingStreamsLock.Unlock()
		case <-ctx.Done():
			log.Fatal().Msg("timeout waiting for processing to finish")
		}
	}
}

// HasNewStreamResponse is the response of HasNewStream.
type HasNewStreamResponse struct {
	HasNewStream bool
	Stream       api.GetStreamsResponseElement
	User         api.GetUserResponse
	PlaybackURL  string
}

// HasNewStream checks if the live stream is online.
func (w *ChannelWatcher) HasNewStream(
	ctx context.Context,
) (res HasNewStreamResponse, err error) {
	log := log.Ctx(ctx)
	res, err = try.DoExponentialBackoffWithResult(
		60,
		30*time.Second,
		2,
		60*time.Minute,
		func() (HasNewStreamResponse, error) {
			streams, err := w.GetStreams(ctx, w.filterChannelID, w.params.PassCode)
			if err != nil {
				if !errors.Is(err, api.HTTPError{}) {
					if err := notifier.NotifyError(ctx, w.filterChannelID, w.params.Labels, err); err != nil {
						log.Err(err).Msg("notify failed")
					}
				}
				return HasNewStreamResponse{}, err
			}
			if len(streams) == 0 {
				return HasNewStreamResponse{
					HasNewStream: false,
				}, nil
			}

			// Find a stream that is online and not being processed.
			var getUserResp api.GetUserResponse
			var playbackURL string
			var stream api.GetStreamsResponseElement
			var lastErr error
			for _, s := range streams {
				if s.Cast.AgencySecret.ChannelName == "" {
					// Stream is scheduled to be live, but not online yet.
					log.Warn().Any("stream", s).Msg("stream is not ready")
					continue
				}

				// Check if stream is an ignored channel.
				if slices.Contains(w.params.Ignore, s.Cast.AgencySecret.ChannelName) {
					continue
				}

				w.processingStreamsLock.Lock()
				_, ok := w.processingStreams[s.UUID]
				w.processingStreamsLock.Unlock()
				if ok {
					// Stream is being processed.
					continue
				}

				// Stream is not being processed, check if it is online.

				channelID := s.Cast.AgencySecret.ChannelName
				log.Info().Str("channelID", channelID).Str("stream", s.Title).Msg("streams found")
				getUserResp, lastErr = w.GetUser(ctx, channelID)
				if lastErr != nil {
					var apiError api.HTTPError
					var isAPIError = errors.As(lastErr, &apiError)
					if !isAPIError || (isAPIError && apiError.Status < 500) {
						if err := notifier.NotifyError(ctx, w.filterChannelID, w.params.Labels, lastErr); err != nil {
							log.Err(err).Msg("notify failed")
						}
					}
					continue
				}

				playbackURL, lastErr = w.GetStreamPlaybackURL(ctx, s.UUID)
				if lastErr != nil {
					if err := notifier.NotifyError(ctx, channelID, w.params.Labels, lastErr); err != nil {
						log.Err(err).Msg("notify failed")
					}
					continue
				}

				stream = s
			}

			if playbackURL == "" {
				return HasNewStreamResponse{
					HasNewStream: false,
				}, lastErr
			}

			return HasNewStreamResponse{
				HasNewStream: true,
				PlaybackURL:  playbackURL,
				User:         getUserResp,
				Stream:       stream,
			}, nil
		},
	)
	if err != nil {
		if err := notifier.NotifyError(ctx, w.filterChannelID, w.params.Labels, err); err != nil {
			log.Err(err).Msg("notify failed")
		}
	}
	return res, err
}

// Process runs the whole preparation, download and post-processing pipeline.
func (w *ChannelWatcher) Process(ctx context.Context, meta api.MetaData, playbackURL string) error {
	log := log.Ctx(ctx)
	channelID := meta.User.Username
	ctx, span := otel.Tracer(tracerName).
		Start(ctx, "withny.Process", trace.WithAttributes(attribute.String("channelID", channelID),
			attribute.Stringer("params", w.params),
		))
	defer span.End()

	metrics.TimeStartRecordingDeferred(channelID)

	span.AddEvent("preparing files")
	state.DefaultState.SetChannelState(
		channelID,
		state.DownloadStatePreparingFiles,
		state.WithLabels(w.params.Labels),
	)
	if err := notifier.NotifyPreparingFiles(ctx, channelID, w.params.Labels, meta); err != nil {
		log.Err(err).Msg("notify failed")
	}

	fnameInfo, err := PrepareFileAutoRename(w.params.OutFormat, meta, w.params.Labels, "info.json")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare info file")
		return err
	}
	var fnameThumb string
	if w.params.Concat {
		fnameThumb, err = PrepareFile(w.params.OutFormat, meta, w.params.Labels, "avif")
	} else {
		fnameThumb, err = PrepareFileAutoRename(w.params.OutFormat, meta, w.params.Labels, "avif")
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fnameStream, err := PrepareFileAutoRename(w.params.OutFormat, meta, w.params.Labels, "ts")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare stream file")
		return err
	}
	fnameChat, err := PrepareFileAutoRename(w.params.OutFormat, meta, w.params.Labels, "chat.json")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare chat file")
		return err
	}
	fnameMuxedExt := strings.ToLower(w.params.RemuxFormat)
	fnameMuxed, err := PrepareFileAutoRename(
		w.params.OutFormat,
		meta,
		w.params.Labels,
		fnameMuxedExt,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare muxed file")
		return err
	}
	fnameAudio, err := PrepareFileAutoRename(w.params.OutFormat, meta, w.params.Labels, "m4a")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare audio file")
		return err
	}
	nameConcatenated, err := FormatOutput(
		w.params.OutFormat,
		meta,
		w.params.Labels,
		"combined."+fnameMuxedExt,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare concatenated file")
		return err
	}
	nameConcatenatedPrefix := strings.TrimSuffix(
		nameConcatenated,
		".combined."+fnameMuxedExt,
	)
	nameAudioConcatenated, err := FormatOutput(
		w.params.OutFormat,
		meta,
		w.params.Labels,
		"combined.m4a",
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to prepare concatenated audio file")
		return err
	}
	nameAudioConcatenatedPrefix := strings.TrimSuffix(
		nameAudioConcatenated,
		".combined.m4a",
	)

	if w.params.WriteMetaDataJSON {
		log.Info().Str("fnameInfo", fnameInfo).Msg("writing info json")
		func() {
			f, err := os.OpenFile(fnameInfo, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				log.Error().Err(err).Msg("failed to open info json")
				return
			}
			defer f.Close()
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(meta); err != nil {
				log.Error().Err(err).Msg("failed to encode meta in info json")
				return
			}
		}()
	}

	if w.params.WriteThumbnail {
		log.Info().Str("fnameThumb", fnameThumb).Msg("writing thumbnail")
		func() {
			url := meta.Stream.ThumbnailURL
			resp, err := w.Get(url)
			if err != nil {
				log.Err(err).Msg("failed to fetch thumbnail")
				return
			}
			defer resp.Body.Close()
			out, err := os.Create(fnameThumb)
			if err != nil {
				log.Err(err).Msg("failed to open thumbnail file")
				return
			}
			defer out.Close()
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				log.Err(err).Msg("failed to download thumbnail file")
				return
			}
		}()
	}

	span.AddEvent("downloading")
	state.DefaultState.SetChannelState(
		channelID,
		state.DownloadStateDownloading,
		state.WithLabels(w.params.Labels),
		state.WithExtra(map[string]interface{}{
			"metadata": meta,
		}),
	)
	if err := notifier.NotifyDownloading(
		ctx,
		channelID,
		w.params.Labels,
		meta,
	); err != nil {
		log.Err(err).Msg("notify failed")
	}

	chatDownloadCtx, chatDownloadCancel := context.WithCancel(ctx)
	if w.params.WriteChat {
		go func() {
			if err := DownloadChat(chatDownloadCtx, api.Scraper{Client: w.Client}, Chat{
				ChannelID:      channelID,
				OutputFileName: fnameChat,
			}); err != nil {
				log.Err(err).Msg("chat download failed")
			}
		}()
	}

	dlErr := DownloadLiveStream(ctx, w.Client, LiveStream{
		MetaData:       meta,
		Params:         w.params,
		OutputFileName: fnameStream,
		PlaybackURL:    playbackURL,
	})
	chatDownloadCancel()

	if errors.Is(dlErr, api.GetPlaybackURLError{}) {
		span.RecordError(dlErr)
		span.SetStatus(codes.Error, dlErr.Error())
		log.Err(dlErr).Msg("get playback url failed")
		return dlErr
	}

	span.AddEvent("post-processing")
	end := metrics.TimeStartRecording(
		ctx,
		metrics.PostProcessing.CompletionTime,
		time.Second,
		metric.WithAttributes(
			attribute.String("channel_id", channelID),
		),
	)
	defer end()
	metrics.PostProcessing.Runs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("channel_id", channelID),
	))
	state.DefaultState.SetChannelState(
		channelID,
		state.DownloadStatePostProcessing,
		state.WithLabels(w.params.Labels),
		state.WithExtra(map[string]interface{}{
			"metadata": meta,
		}),
	)
	if err := notifier.NotifyPostProcessing(
		ctx,
		channelID,
		w.params.Labels,
		meta,
	); err != nil {
		log.Err(err).Msg("notify failed")
	}
	log.Info().Msg("post-processing...")

	var remuxErr error

	probeErr := probe.Do([]string{fnameStream}, probe.WithQuiet())
	if probeErr != nil {
		log.Error().Err(probeErr).Msg("ts is unreadable by ffmpeg")
		if w.params.DeleteCorrupted {
			if err := os.Remove(fnameStream); err != nil {
				log.Error().
					Str("path", fnameStream).
					Err(err).
					Msg("failed to remove corrupted file")
			}
		}
	}
	if w.params.Remux && probeErr == nil {
		log.Info().Str("output", fnameMuxed).Str("input", fnameStream).Msg(
			"remuxing stream...",
		)
		remuxErr = remux.Do(ctx, fnameMuxed, fnameStream)
		if remuxErr != nil {
			log.Error().Err(remuxErr).Msg("ffmpeg remux finished with error")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", channelID),
			))
		}
	}
	var extractAudioErr error
	// Extract audio if remux on, or when concat is ofw.
	if w.params.ExtractAudio && (!w.params.Concat || w.params.Remux) && probeErr == nil {
		log.Info().Str("output", fnameAudio).Str("input", fnameStream).Msg(
			"extrating audio...",
		)
		extractAudioErr = remux.Do(ctx, fnameAudio, fnameStream, remux.WithAudioOnly())
		if extractAudioErr != nil {
			log.Error().Err(extractAudioErr).Msg("ffmpeg audio extract finished with error")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", channelID),
			))
		}
	}

	// Concat
	if w.params.Concat {
		log.Info().Str("output", nameConcatenated).Str("prefix", nameConcatenatedPrefix).Msg(
			"concatenating stream...",
		)
		concatOpts := []concat.Option{
			concat.IgnoreExtension(),
		}
		if concatErr := concat.WithPrefix(ctx, w.params.RemuxFormat, nameConcatenatedPrefix, concatOpts...); concatErr != nil {
			log.Error().Err(concatErr).Msg("ffmpeg concat finished with error")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", channelID),
			))
		}

		if w.params.ExtractAudio {
			log.Info().
				Str("output", nameAudioConcatenated).
				Str("prefix", nameAudioConcatenatedPrefix).
				Msg(
					"concatenating audio stream...",
				)
			concatOpts = append(concatOpts, concat.WithAudioOnly())
			if concatErr := concat.WithPrefix(ctx, "m4a", nameAudioConcatenatedPrefix, concatOpts...); concatErr != nil {
				log.Error().Err(concatErr).Msg("ffmpeg concat finished with error")
				metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
					attribute.String("channel_id", channelID),
				))
			}
		}
	}

	// Delete intermediates
	if !w.params.KeepIntermediates && w.params.Remux &&
		probeErr == nil &&
		remuxErr == nil &&
		extractAudioErr == nil {
		log.Info().Str("file", fnameStream).Msg("delete intermediate files")
		if err := os.Remove(fnameStream); err != nil {
			log.Err(err).Msg("couldn't delete intermediate file")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", channelID),
			))
		}
	}

	span.AddEvent("done")
	log.Info().Msg("done")

	return dlErr
}
