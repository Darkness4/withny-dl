// package withny provides a way to watch a withny channel.
package withny

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Darkness4/withny-dl/notify/notifier"
	"github.com/Darkness4/withny-dl/state"
	"github.com/Darkness4/withny-dl/telemetry/metrics"
	"github.com/Darkness4/withny-dl/utils/try"
	"github.com/Darkness4/withny-dl/video/concat"
	"github.com/Darkness4/withny-dl/video/probe"
	"github.com/Darkness4/withny-dl/video/remux"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName    = "withny"
	msgBufMax     = 100
	errBufMax     = 10
	commentBufMax = 100
)

var (
	// ErrLiveStreamNotOnline is returned when the live stream is not online.
	ErrLiveStreamNotOnline = errors.New("live stream is not online")
)

// ChannelWatcher is responsible to watch a withny channel.
type ChannelWatcher struct {
	*api.Client
	params    *Params
	channelID string
	log       *zerolog.Logger
}

// NewChannelWatcher creates a new withny channel watcher.
func NewChannelWatcher(client *api.Client, params *Params, channelID string) *ChannelWatcher {
	if client == nil {
		log.Panic().Msg("client is nil")
	}
	logger := log.With().Str("channelID", channelID).Logger()
	return &ChannelWatcher{
		Client:    client,
		params:    params,
		channelID: channelID,
		log:       &logger,
	}
}

// Watch watches the channel for any new live stream.
func (w *ChannelWatcher) Watch(ctx context.Context) (api.MetaData, error) {
	w.log.Info().Any("params", w.params).Msg("watching channel")

	online, streams, err := w.IsOnline(ctx)
	if err != nil {
		return api.MetaData{}, err
	}

	if !online {
		if !w.params.WaitForLive {
			return api.MetaData{}, ErrLiveStreamNotOnline
		}
		streams = func() api.GetStreamsResponse {
			ticker := time.NewTicker(w.params.WaitPollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return streams
				case <-ticker.C:
					if online, streams, err := w.IsOnline(ctx); err != nil {
						log.Err(err).Msg("failed to check if online")
					} else if online {
						return streams
					}
				}
			}
		}()
	}

	getUserResp, err := w.Client.GetUser(ctx, w.channelID)
	if err != nil {
		return api.MetaData{}, err
	}

	meta := api.MetaData{
		User:   getUserResp,
		Stream: streams[0],
	}

	err = w.Process(ctx, meta)
	return meta, err
}

// IsOnline checks if the live stream is online.
func (w *ChannelWatcher) IsOnline(
	ctx context.Context,
) (ok bool, streams api.GetStreamsResponse, err error) {
	type resT = struct {
		ok      bool
		streams api.GetStreamsResponse
	}
	res, err := try.DoExponentialBackoffWithContextAndResult(
		ctx,
		5,
		30*time.Second,
		2,
		5*time.Minute,
		func(ctx context.Context) (resT, error) {
			streams, err := w.GetStreams(ctx, w.channelID)
			return resT{
				ok:      len(streams) > 0,
				streams: streams,
			}, err
		},
	)
	return res.ok, res.streams, err
}

// Process runs the whole preparation, download and post-processing pipeline.
func (w *ChannelWatcher) Process(ctx context.Context, meta api.MetaData) error {
	ctx, span := otel.Tracer(tracerName).
		Start(ctx, "withny.Process", trace.WithAttributes(attribute.String("channelID", w.channelID),
			attribute.Stringer("params", w.params),
		))
	defer span.End()

	metrics.TimeStartRecordingDeferred(w.channelID)

	span.AddEvent("preparing files")
	state.DefaultState.SetChannelState(
		w.channelID,
		state.DownloadStatePreparingFiles,
		state.WithLabels(w.params.Labels),
	)
	if err := notifier.NotifyPreparingFiles(ctx, w.channelID, w.params.Labels, meta); err != nil {
		log.Err(err).Msg("notify failed")
	}

	fnameInfo, err := PrepareFile(w.params.OutFormat, meta, w.params.Labels, "info.json")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fnameThumb, err := PrepareFile(w.params.OutFormat, meta, w.params.Labels, "png")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fnameStream, err := PrepareFile(w.params.OutFormat, meta, w.params.Labels, "ts")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fnameChat, err := PrepareFile(w.params.OutFormat, meta, w.params.Labels, "chat.json")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fnameMuxedExt := strings.ToLower(w.params.RemuxFormat)
	fnameMuxed, err := PrepareFile(w.params.OutFormat, meta, w.params.Labels, fnameMuxedExt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fnameAudio, err := PrepareFile(w.params.OutFormat, meta, w.params.Labels, "m4a")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
		return err
	}
	nameAudioConcatenatedPrefix := strings.TrimSuffix(
		nameAudioConcatenated,
		".combined.m4a",
	)

	if w.params.WriteMetaDataJSON {
		w.log.Info().Str("fnameInfo", fnameInfo).Msg("writing info json")
		func() {
			b, err := json.MarshalIndent(meta, "", "  ")
			if err != nil {
				w.log.Error().Err(err).Msg("failed to marshal meta")
				return
			}
			if err := os.WriteFile(fnameInfo, b, 0o755); err != nil {
				w.log.Error().Err(err).Msg("failed to write meta in info json")
				return
			}
		}()
	}

	if w.params.WriteThumbnail {
		w.log.Info().Str("fnameThumb", fnameThumb).Msg("writing thunnail")
		func() {
			url := meta.Stream.ThumbnailURL
			resp, err := w.Get(url)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to fetch thumbnail")
				return
			}
			defer resp.Body.Close()
			out, err := os.Create(fnameThumb)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to open thumbnail file")
				return
			}
			defer out.Close()
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to download thumbnail file")
				return
			}
		}()
	}

	span.AddEvent("downloading")
	state.DefaultState.SetChannelState(
		w.channelID,
		state.DownloadStateDownloading,
		state.WithLabels(w.params.Labels),
		state.WithExtra(map[string]interface{}{
			"metadata": meta,
		}),
	)
	if err := notifier.NotifyDownloading(
		ctx,
		w.channelID,
		w.params.Labels,
		meta,
	); err != nil {
		log.Err(err).Msg("notify failed")
	}

	chatDownloadCtx, chatDownloadCancel := context.WithCancel(ctx)
	if w.params.WriteChat {
		go func() {
			if err := DownloadChat(chatDownloadCtx, w.Client, Chat{
				ChannelID:      w.channelID,
				OutputFileName: fnameChat,
			}); err != nil {
				w.log.Error().Err(err).Msg("chat download failed")
			}
		}()
	}

	dlErr := DownloadLiveStream(ctx, w.Client, LiveStream{
		MetaData:       meta,
		Params:         w.params,
		OutputFileName: fnameStream,
	})

	chatDownloadCancel()

	span.AddEvent("post-processing")
	end := metrics.TimeStartRecording(
		ctx,
		metrics.PostProcessing.CompletionTime,
		time.Second,
		metric.WithAttributes(
			attribute.String("channel_id", w.channelID),
		),
	)
	defer end()
	metrics.PostProcessing.Runs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("channel_id", w.channelID),
	))
	state.DefaultState.SetChannelState(
		w.channelID,
		state.DownloadStatePostProcessing,
		state.WithLabels(w.params.Labels),
		state.WithExtra(map[string]interface{}{
			"metadata": meta,
		}),
	)
	if err := notifier.NotifyPostProcessing(
		ctx,
		w.channelID,
		w.params.Labels,
		meta,
	); err != nil {
		log.Err(err).Msg("notify failed")
	}
	w.log.Info().Msg("post-processing...")

	var remuxErr error

	probeErr := probe.Do([]string{fnameStream}, probe.WithQuiet())
	if probeErr != nil {
		w.log.Error().Err(probeErr).Msg("ts is unreadable by ffmpeg")
		if w.params.DeleteCorrupted {
			if err := os.Remove(fnameStream); err != nil {
				w.log.Error().
					Str("path", fnameStream).
					Err(err).
					Msg("failed to remove corrupted file")
			}
		}
	}
	if w.params.Remux && probeErr == nil {
		w.log.Info().Str("output", fnameMuxed).Str("input", fnameStream).Msg(
			"remuxing stream...",
		)
		remuxErr = remux.Do(ctx, fnameMuxed, fnameStream)
		if remuxErr != nil {
			w.log.Error().Err(remuxErr).Msg("ffmpeg remux finished with error")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", w.channelID),
			))
		}
	}
	var extractAudioErr error
	// Extract audio if remux on, or when concat is ofw.
	if w.params.ExtractAudio && (!w.params.Concat || w.params.Remux) && probeErr == nil {
		w.log.Info().Str("output", fnameAudio).Str("input", fnameStream).Msg(
			"extrating audio...",
		)
		extractAudioErr = remux.Do(ctx, fnameAudio, fnameStream, remux.WithAudioOnly())
		if extractAudioErr != nil {
			w.log.Error().Err(extractAudioErr).Msg("ffmpeg audio extract finished with error")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", w.channelID),
			))
		}
	}

	// Concat
	if w.params.Concat {
		w.log.Info().Str("output", nameConcatenated).Str("prefix", nameConcatenatedPrefix).Msg(
			"concatenating stream...",
		)
		concatOpts := []concat.Option{
			concat.IgnoreExtension(),
		}
		if concatErr := concat.WithPrefix(ctx, w.params.RemuxFormat, nameConcatenatedPrefix, concatOpts...); concatErr != nil {
			w.log.Error().Err(concatErr).Msg("ffmpeg concat finished with error")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", w.channelID),
			))
		}

		if w.params.ExtractAudio {
			w.log.Info().
				Str("output", nameAudioConcatenated).
				Str("prefix", nameAudioConcatenatedPrefix).
				Msg(
					"concatenating audio stream...",
				)
			concatOpts = append(concatOpts, concat.WithAudioOnly())
			if concatErr := concat.WithPrefix(ctx, "m4a", nameAudioConcatenatedPrefix, concatOpts...); concatErr != nil {
				w.log.Error().Err(concatErr).Msg("ffmpeg concat finished with error")
				metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
					attribute.String("channel_id", w.channelID),
				))
			}
		}
	}

	// Delete intermediates
	if !w.params.KeepIntermediates && w.params.Remux &&
		probeErr == nil &&
		remuxErr == nil &&
		extractAudioErr == nil {
		w.log.Info().Str("file", fnameStream).Msg("delete intermediate files")
		if err := os.Remove(fnameStream); err != nil {
			w.log.Error().Err(err).Msg("couldn't delete intermediate file")
			metrics.PostProcessing.Errors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("channel_id", w.channelID),
			))
		}
	}

	span.AddEvent("done")
	w.log.Info().Msg("done")

	return dlErr
}
