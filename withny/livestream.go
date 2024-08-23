package withny

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/Darkness4/withny-dl/hls"
	"github.com/Darkness4/withny-dl/telemetry/metrics"
	"github.com/Darkness4/withny-dl/utils/try"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// LiveStream encapsulates the withny live stream.
type LiveStream struct {
	MetaData       api.MetaData
	Params         *Params
	OutputFileName string
}

// DownloadLiveStream downloads a withny live stream.
func DownloadLiveStream(ctx context.Context, client *api.Client, ls LiveStream) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "withny.downloadStream", trace.WithAttributes(
		attribute.String("channel_id", ls.MetaData.User.Username),
		attribute.String("fname", ls.OutputFileName),
	))
	defer span.End()

	// Fetch playlist
	playbackURL, err := client.GetStreamPlaybackURL(ctx, ls.MetaData.Stream.UUID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to fetch playback URL")
		return err
	}

	playlists, err := client.GetPlaylists(ctx, playbackURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to fetch playlists")
		return err
	}
	if len(playlists) == 0 {
		err := errors.New("no playlists found")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("no playlists found")
		return err
	}

	var downloader *hls.Downloader
	constraint := ls.Params.QualityConstraint
	for {
		playlist, ok := api.GetBestPlaylist(playlists, constraint)
		if !ok {
			log.Warn().
				Any("playlists", playlists).
				Any("fallback", playlists[0]).
				Any("constraint", constraint).
				Msg("no playlist found with current constraint")
			playlist = playlists[0]
		}

		downloader = hls.NewDownloader(
			client,
			&log.Logger,
			ls.Params.PacketLossMax,
			playlist.URL,
		)

		if ok, err := try.DoWithResult(5, 5*time.Second, func() (bool, error) {
			return downloader.Probe(ctx)
		}); !ok || err != nil {
			log.Warn().Err(err).Msg("failed to fetch playlist, switching to next playlist")
			constraint.Ignored = append(constraint.Ignored, playlist.URL)
		}

		if ok {
			log.Info().Any("playlist", playlist).Msg("received new HLS info")
			span.AddEvent("playlist received", trace.WithAttributes(
				attribute.String("url", playlist.URL),
				attribute.String("format", playlist.Video),
			))
			break
		}
	}

	metrics.TimeEndRecording(
		ctx,
		metrics.Downloads.InitTime,
		ls.MetaData.User.Username,
		metric.WithAttributes(
			attribute.String("channel_id", ls.MetaData.User.Username),
		),
	)

	span.AddEvent("downloading")
	end := metrics.TimeStartRecording(
		ctx,
		metrics.Downloads.CompletionTime,
		time.Second,
		metric.WithAttributes(
			attribute.String("channel_id", ls.MetaData.User.Username),
		),
	)
	defer end()
	metrics.Downloads.Runs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("channel_id", ls.MetaData.User.Username),
	))

	// Actually download. It will block until the download is finished.
	file, err := os.Create(ls.OutputFileName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to create file")
		return err
	}
	defer file.Close()

	if err = downloader.Read(ctx, file); err != nil && !errors.Is(err, io.EOF) &&
		!errors.Is(err, context.Canceled) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Err(err).Msg("failed to download")
		return err
	}

	span.AddEvent("done")
	log.Info().Msg("done")
	return nil
}
