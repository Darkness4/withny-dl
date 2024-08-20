package withny

import (
	"context"
	"os"
	"time"

	"github.com/Darkness4/withny-dl/hls"
	"github.com/Darkness4/withny-dl/telemetry/metrics"
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
		return err
	}

	playlists, err := client.GetPlaylists(ctx, playbackURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	playlist, ok := api.GetBestPlaylist(playlists, ls.Params.QualityConstraint)
	if !ok {
		log.Warn().
			Any("playlists", playlists).
			Any("fallback", playlists[0]).
			Any("constraint", ls.Params.QualityConstraint).
			Msg("no playlist found with current constraint")
		playlist = playlists[0]
	}

	log.Info().Any("playlist", playlist).Msg("received new HLS info")
	span.AddEvent("playlist received", trace.WithAttributes(
		attribute.String("url", playlist.URL),
		attribute.String("format", playlist.Video),
	))
	metrics.TimeEndRecording(
		ctx,
		metrics.Downloads.InitTime,
		ls.MetaData.User.Username,
		metric.WithAttributes(
			attribute.String("channel_id", ls.MetaData.User.Username),
		),
	)

	downloader := hls.NewDownloader(
		client,
		&log.Logger,
		ls.Params.PacketLossMax,
		playlist.URL,
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
		return err
	}
	defer file.Close()

	if err = downloader.Read(ctx, file); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.AddEvent("done")
	log.Info().Msg("done")
	return nil
}
