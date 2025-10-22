package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"time"

	"github.com/Darkness4/withny-dl/socketio"
	"github.com/Darkness4/withny-dl/utils/strings"
	"github.com/coder/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const socketIOURL = "wss://api.withny.fun/socket.io/"

// SessionWebSocket is used to interact with the withny Socket.io session websocket.
type SessionWebSocket struct {
	*Client
	url        *neturl.URL
	streamUUID string
	passCode   string

	log *zerolog.Logger
}

// NewSessionWebSocket creates a new WebSocket.
func NewSessionWebSocket(
	client *Client,
	streamUUID string,
	passCode string,
) *SessionWebSocket {
	censoredPassCode := strings.Censor(passCode, 4, "*")
	logger := log.With().Str("streamUUID", streamUUID).Str("passCode", censoredPassCode).Logger()
	u, err := neturl.Parse(socketIOURL)
	if err != nil {
		panic(err)
	}
	u.Scheme = "wss"
	if passCode == "" {
		// This is a bug from withny
		passCode = "undefined"
	}
	w := &SessionWebSocket{
		Client:     client,
		url:        u,
		streamUUID: streamUUID,
		passCode:   passCode,
		log:        &logger,
	}
	return w
}

// Dial connects to the WebSocket server.
func (w *SessionWebSocket) Dial(ctx context.Context) (*websocket.Conn, error) {
	// Build header query which is the base64 encoded value of the json of authorization and host.
	creds, err := w.credentialsCache.Get()
	if err != nil {
		w.log.Err(err).Msg("failed to get cached credentials")
	}
	q := w.url.Query()
	q.Set("uuid", w.streamUUID)
	q.Set("token", creds.Token)
	q.Set("passCode", w.passCode)
	q.Set("EIO", "4")
	q.Set("transport", "websocket")
	w.url.RawQuery = q.Encode()

	// Connect to the websocket server
	conn, _, err := websocket.Dial(ctx, w.url.String(), &websocket.DialOptions{
		HTTPClient: w.Client.Client,
		HTTPHeader: map[string][]string{
			"Origin": {"https://www.withny.fun"},
		},
	})
	if err != nil {
		w.log.Err(err).Msg("failed to dial websocket")
		return nil, err
	}
	conn.SetReadLimit(10485760) // 10 MiB
	return conn, nil
}

// Watch listens on the WebSocket.
func (w *SessionWebSocket) Watch(
	ctx context.Context,
	conn *websocket.Conn,
	streams chan<- *GetStreamsResponseElement,
) error {
	// Connection init
	go func() {
		if err := w.ConnectionInit(ctx, conn); err != nil {
			w.log.Err(err).Msg("failed to init connection")
		}
	}()

	// Start listening for messages from the websocket server
	for {
		msgType, msg, err := conn.Read(ctx)
		if err != nil {
			var closeError websocket.CloseError
			if errors.As(err, &closeError) {
				if closeError.Code == websocket.StatusNormalClosure {
					w.log.Info().Msg("websocket closed cleanly")
					return io.EOF
				}
			}
			return err
		}
		switch msgType {
		case websocket.MessageText:
			decoded, err := socketio.UnmarshalV4(msg)
			if err != nil {
				w.log.Trace().Err(err).Str("msg", string(msg)).Msg("failed to unmarshal message")
				continue
			}

			// We only want one thing: the stream metadata. So we do a precise parsing.

			var payload []json.RawMessage
			if err := json.Unmarshal(decoded.Payload, &payload); err != nil {
				w.log.Trace().Err(err).Any("msg", decoded).Msg("failed to unmarshal payload")
				continue
			}
			if len(payload) != 2 {
				w.log.Trace().
					Any("msg", decoded).
					Any("payload", payload).
					Msg("ignoring unwanted payload (wrong size)")
				continue
			}
			var typ string
			if err := json.Unmarshal(payload[0], &typ); err != nil {
				w.log.Err(err).Any("msg", decoded).Msg("failed to unmarshal payload type")
				continue
			}
			if typ != "stream" {
				w.log.Trace().
					Any("msg", decoded).
					Str("type", typ).
					Msg("ignoring unwanted payload (wrong type)")
				continue
			}

			var stream GetStreamsResponseElement
			if err := json.Unmarshal(payload[1], &stream); err != nil {
				w.log.Err(err).Any("msg", decoded).Msg("failed to unmarshal payload")
				continue
			}
			streams <- &stream
		}
	}
}

// ConnectionInit initializes the connection to the WebSocket.
func (w *SessionWebSocket) ConnectionInit(ctx context.Context, conn *websocket.Conn) error {
	return conn.Write(ctx, websocket.MessageText, []byte(`40/channels,`))
}

// FetchStreamMetadataSync will fetch the stream metadata from the server synchronously.
//
// It will hang for 5 seconds before timing out.
func FetchStreamMetadataSync(
	ctx context.Context,
	client *Client,
	streamUUID string,
	passCode string,
) (*GetStreamsResponseElement, error) {
	ws := NewSessionWebSocket(client, streamUUID, passCode)
	conn, err := ws.Dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to dial websocket: %w", err)
	}

	streamsCh := make(chan *GetStreamsResponseElement, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ws.Watch(ctx, conn, streamsCh)
	}()
	select {
	case err := <-errCh:
		return nil, err
	case stream := <-streamsCh:
		return stream, nil
	case <-time.After(5 * time.Second):
		return nil, ErrStreamNotFound
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
