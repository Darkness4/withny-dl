package withny

import (
	"context"
	"encoding/json"
	"os"

	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Chat encapsulates the withny chat.
type Chat struct {
	ChannelID      string
	OutputFileName string
}

// DownloadChat downloads a withny chat.
func DownloadChat(ctx context.Context, client *api.Client, chat Chat) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "withny.downloadChat", trace.WithAttributes(
		attribute.String("channel_id", chat.ChannelID),
		attribute.String("fname", chat.OutputFileName),
	))
	defer span.End()

	endpoint, suuid, err := api.NewScraper(client).
		FetchGraphQLAndStreamUUID(ctx, chat.ChannelID)
	if err != nil {
		log.Err(err).Msg("failed to find graphql endpoint for chat")
		return err
	}

	ws := api.NewWebSocket(client, endpoint)
	conn, err := ws.Dial(ctx)
	if err != nil {
		log.Err(err).Msg("failed to dial websocket")
		return err
	}

	commentsCh := make(chan *api.Comment, commentBufMax)
	defer close(commentsCh)
	go func() {
		file, err := os.Create(chat.OutputFileName)
		if err != nil {
			log.Err(err).Msg("failed to create file, cannot write comments")
			return
		}
		defer file.Close()

		if _, err := file.WriteString("[\n"); err != nil {
			log.Err(err).Msg("failed to write comment")
			return
		}

		for comment := range commentsCh {
			jsonData, err := json.Marshal(comment)
			if err != nil {
				log.Err(err).Msg("failed to marshal comment")
				continue
			}
			if _, err := file.Write(jsonData); err != nil {
				log.Err(err).Msg("failed to write comment")
			}
			if _, err := file.WriteString(",\n"); err != nil {
				log.Err(err).Msg("failed to write comment")
			}
		}

		if _, err := file.WriteString("]\n"); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			log.Err(err).Msg("failed to write comment")
			return
		}
	}()
	err = ws.WatchComments(ctx, conn, suuid, commentsCh)
	if err != nil {
		log.Err(err).Msg("failed to watch comments")
		return err
	}
	return nil
}
