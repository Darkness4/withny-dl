// Package withny provides a way to watch a withny channel.
package withny

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Darkness4/withny-dl/notify/notifier"
	"github.com/Darkness4/withny-dl/state"
	"github.com/Darkness4/withny-dl/telemetry/metrics"
	"github.com/Darkness4/withny-dl/utils/sync"
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
	*api.Scraper
	params *Params
	// filterChannelID is like a channelID, but an empty one will select all channels.
	filterChannelID string
	// processingStreams is a set of streamsIDs that are currently being processed.
	processingStreams *sync.Set[string]
}

// NewChannelWatcher creates a new withny channel watcher.
func NewChannelWatcher(scraper *api.Scraper, params *Params, channelID string) *ChannelWatcher {
	if scraper == nil {
		log.Panic().Msg("scraper is nil")
	}
	return &ChannelWatcher{
		Scraper:           scraper,
		params:            params,
		filterChannelID:   channelID,
		processingStreams: sync.NewSet[string](),
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

		w.processingStreams.Set(res.Stream.UUID)

		go func() {
			defer w.processingStreams.Release(res.Stream.UUID)
			log := log.With().
				Str("channelID", res.User.Username).
				Str("streamID", res.Stream.UUID).
				Logger()
			ctx = log.WithContext(ctx)

			err := w.Process(ctx, api.MetaData{
				User:   res.User,
				Stream: res.Stream,
			}, res.Playlists)

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
			if w.processingStreams.Len() == 0 {
				return
			}
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
	Playlists    []api.Playlist
}

// HasNewStream checks if the live stream is online.
func (w *ChannelWatcher) HasNewStream(
	ctx context.Context,
) (res HasNewStreamResponse, err error) {
	res, err = try.DoExponentialBackoffWithResult(
		60,
		30*time.Second,
		2,
		60*time.Minute,
		func() (HasNewStreamResponse, error) {
			if w.filterChannelID == "" || w.params.PassCode == "" {
				// use this logic when we want to download any channel
				return w.hasNewStreamMethodAPI(ctx, w.filterChannelID)
			}
			// use this logic when we want to download a specific channel
			return w.hasNewStreamMethodScrape(ctx, w.filterChannelID)
		},
	)
	return res, err
}

func (w *ChannelWatcher) hasNewStreamMethodAPI(
	ctx context.Context,
	filterChannelID string,
) (HasNewStreamResponse, error) {
	streams, err := w.GetStreams(ctx, filterChannelID, w.params.PassCode)
	if err != nil {
		if !errors.Is(err, api.HTTPError{}) {
			if err := notifier.NotifyError(ctx, filterChannelID, w.params.Labels, err); err != nil {
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

		if w.processingStreams.Contains(s.UUID) {
			continue
		}

		channelID := s.Cast.AgencySecret.ChannelName
		log.Info().Str("channelID", channelID).Str("stream", s.Title).Msg("streams found")

		res, err := w.validateAndFetchStreamData(ctx, channelID, s.UUID)
		if err != nil {
			lastErr = err
			continue
		}

		if res.HasNewStream {
			res.Stream = s
			return res, nil
		}
	}

	return HasNewStreamResponse{}, lastErr
}

func (w *ChannelWatcher) hasNewStreamMethodScrape(
	ctx context.Context,
	channelID string,
) (HasNewStreamResponse, error) {
	suuid, err := w.FetchStreamUUID(ctx, channelID, w.params.PassCode)
	if err != nil {
		err := fmt.Errorf(
			"failed to check if channel %s has a new stream: %w",
			channelID,
			err,
		)
		if err := notifier.NotifyError(ctx, "", w.params.Labels, err); err != nil {
			log.Err(err).Msg("notify failed")
		}
		return HasNewStreamResponse{}, err
	}

	stream, err := api.FetchStreamMetadataSync(ctx, w.Client, suuid, w.params.PassCode)
	if errors.Is(err, api.ErrStreamNotFound) {
		return HasNewStreamResponse{}, nil
	} else if err != nil {
		return HasNewStreamResponse{}, fmt.Errorf("failed to fetch stream metadata: %w", err)
	}

	if w.processingStreams.Contains(stream.UUID) {
		return HasNewStreamResponse{}, nil
	}

	res, err := w.validateAndFetchStreamData(ctx, channelID, stream.UUID)
	if err != nil {
		return HasNewStreamResponse{}, err
	}
	if res.HasNewStream {
		res.Stream = stream
	}

	return res, nil
}

// validateAndFetchStreamData fetches and validates user, playback URL, and playlists for a stream.
func (w *ChannelWatcher) validateAndFetchStreamData(
	ctx context.Context,
	channelID string,
	streamUUID string,
) (HasNewStreamResponse, error) {
	getUserResp, err := w.GetUser(ctx, channelID)
	if err != nil {
		err = fmt.Errorf("failed to fetch user metadata: %w", err)
		w.notifyOnClientOrUnknownError(ctx, channelID, err)
		return HasNewStreamResponse{}, err
	}

	playbackURL, err := w.GetStreamPlaybackURL(ctx, streamUUID)
	if err != nil {
		err = fmt.Errorf("failed to fetch stream playback url: %w", err)
		w.notifyOn403OrUnknownError(ctx, channelID, err)
		return HasNewStreamResponse{}, err
	}

	playlists, err := w.GetPlaylists(ctx, playbackURL, w.params.PlaylistRetries)
	if err != nil {
		err = fmt.Errorf("failed to fetch playlists: %w", err)

		var apiError api.HTTPError
		if errors.As(err, &apiError) && apiError.Status == 404 {
			// The stream is not online yet.
			return HasNewStreamResponse{}, nil
		}

		w.notifyOn403OrUnknownError(ctx, channelID, err)
		return HasNewStreamResponse{}, err
	}
	if len(playlists) == 0 {
		return HasNewStreamResponse{}, nil
	}

	return HasNewStreamResponse{
		HasNewStream: true,
		Playlists:    playlists,
		User:         getUserResp,
	}, nil
}

// notifyOnClientOrUnknownError sends notifications for client errors (< 500) or non-API errors.
func (w *ChannelWatcher) notifyOnClientOrUnknownError(
	ctx context.Context,
	channelID string,
	err error,
) {
	var apiError api.HTTPError
	isAPIError := errors.As(err, &apiError)

	// Only notify on client error or unknown error
	if !isAPIError || (isAPIError && apiError.Status < 500) {
		if notifyErr := notifier.NotifyError(ctx, channelID, w.params.Labels, err); notifyErr != nil {
			log.Err(notifyErr).Msg("notify failed")
		}
	}
}

// notifyOn403OrUnknownError sends notifications for 403 errors or non-API errors.
func (w *ChannelWatcher) notifyOn403OrUnknownError(
	ctx context.Context,
	channelID string,
	err error,
) {
	log := log.Ctx(ctx)
	var apiError api.HTTPError
	isAPIError := errors.As(err, &apiError)

	// Only notify on 403 or unknown error
	if !isAPIError || (isAPIError && apiError.Status == 403) {
		if notifyErr := notifier.NotifyError(ctx, channelID, w.params.Labels, err); notifyErr != nil {
			log.Err(notifyErr).Msg("notify failed")
		}
	}
}

// Process runs the whole preparation, download and post-processing pipeline.
func (w *ChannelWatcher) Process(
	ctx context.Context,
	meta api.MetaData,
	playlists []api.Playlist,
) error {
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
		state.WithExtra(map[string]any{
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
		Playlists:      playlists,
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
		state.WithExtra(map[string]any{
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
