package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"strings"

	"github.com/Darkness4/withny-dl/graphql"
	"github.com/coder/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const queryFormat = `subscription MySubscription {
	onPostComment(streamUUID: "%s") {
		streamUUID
		commentUUID
		userUUID
		username
		name
		contentType
		content
		tipAmount
		itemID
		itemName
		itemURI
		animationURI
		itemPower
		itemLifetime
		createdAt
		updatedAt
		deletedAt
	}
}`

// WebSocket is used to interact with the withny WebSocket.
type WebSocket struct {
	*Client
	url         *neturl.URL
	realtimeURL *neturl.URL
	log         *zerolog.Logger
}

// WSResponse is the response from the WebSocket.
type WSResponse struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

// NewWebSocket creates a new WebSocket.
func NewWebSocket(
	client *Client,
	url string,
) *WebSocket {
	logger := log.With().Str("url", url).Logger()
	u, err := neturl.Parse(url)
	if err != nil {
		panic(err)
	}
	rtURL, err := neturl.Parse(strings.Replace(url, "appsync-api", "appsync-realtime-api", 1))
	if err != nil {
		panic(err)
	}
	u.Scheme = "wss"
	w := &WebSocket{
		Client:      client,
		url:         u,
		realtimeURL: rtURL,
		log:         &logger,
	}
	return w
}

// Dial connects to the WebSocket server.
func (w *WebSocket) Dial(ctx context.Context) (*websocket.Conn, error) {
	// Build header query which is the base64 encoded value of the json of authorization and host.
	creds, err := w.Client.credentialsCache.Get()
	if err != nil {
		w.log.Err(err).Msg("failed to get cached credentials")
	}
	v := map[string]string{
		"Authorization": "Bearer " + creds.Token,
		"Host":          w.url.Host,
	}
	vjson, err := json.Marshal(v)
	if err != nil {
		w.log.Err(err).Msg("failed to marshal header")
		return nil, err
	}
	vb64 := base64.StdEncoding.EncodeToString(vjson)

	q := w.realtimeURL.Query()
	q.Set("header", vb64)
	q.Set("payload", "e30=")
	w.realtimeURL.RawQuery = q.Encode()

	// Connect to the websocket server
	conn, _, err := websocket.Dial(ctx, w.realtimeURL.String(), &websocket.DialOptions{
		HTTPClient: w.Client.Client,
		HTTPHeader: map[string][]string{
			"Origin": {"https://www.withny.fun"},
		},
		Subprotocols: []string{"graphql-ws"},
	})
	if err != nil {
		w.log.Err(err).Msg("failed to dial websocket")
		return nil, err
	}
	conn.SetReadLimit(10485760) // 10 MiB
	return conn, nil
}

// WatchComments listens for comments on the WebSocket.
func (w *WebSocket) WatchComments(
	ctx context.Context,
	conn *websocket.Conn,
	streamID string,
	commentChan chan<- *Comment,
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
			w.log.Trace().Str("msg", string(msg)).Msg("ws receive")
			var msgObj WSResponse
			if err := json.Unmarshal(msg, &msgObj); err != nil {
				w.log.Error().Str("msg", string(msg)).Err(err).Msg("failed to decode")
				continue
			}

			switch msgObj.Type {
			case "connection_ack":
				w.log.Info().Msg("ws fully connected")
				// Subscribe to comments
				go func() {
					if err := w.Subscribe(ctx, conn, streamID); err != nil {
						w.log.Err(err).Msg("failed to subscribe")
					}
				}()
			case "start_ack":
				w.log.Info().Msg("subscription started")
			case "data":
				var resp WSCommentResponse
				if err := json.Unmarshal(msgObj.Payload, &resp); err != nil {
					w.log.Err(err).Msg("failed to decode comment")
					continue
				}
				commentChan <- &resp.Data.OnPostComment
			case "ka":
				// It's a keep alive message!
			default:
				w.log.Warn().
					Str("type", msgObj.Type).
					Str("msg", string(msg)).
					Msg("received unhandled msg type")
			}

		default:
			w.log.Error().
				Int("type", int(msgType)).
				Str("msg", string(msg)).
				Msg("received unhandled msg type")
		}
	}
}

// ConnectionInit initializes the connection to the WebSocket.
func (w *WebSocket) ConnectionInit(ctx context.Context, conn *websocket.Conn) error {
	initMsgJSON, err := json.Marshal(graphql.ConnectionInit)
	if err != nil {
		w.log.Err(err).Msg("failed to marshal connection init")
		return err
	}
	return conn.Write(ctx, websocket.MessageText, initMsgJSON)
}

// Subscribe subscribes to the WebSocket.
func (w *WebSocket) Subscribe(ctx context.Context, conn *websocket.Conn, streamID string) error {
	query := graphql.Query{
		Query:     fmt.Sprintf(queryFormat, streamID),
		Variables: map[string]interface{}{},
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		w.log.Err(err).Msg("failed to marshal query")
		return err
	}
	creds, err := w.Client.credentialsCache.Get()
	if err != nil {
		w.log.Err(err).Msg("failed to get cached credentials")
	}
	msg := graphql.BuildSubscribeMessage(graphql.SubscribeMessagePayload{
		Data: string(jsonQuery),
		Extensions: map[string]interface{}{
			"authorization": map[string]string{
				"Authorization": creds.TokenType + " " + creds.Token,
				"host":          w.url.Host,
			},
		},
	})
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		w.log.Err(err).Msg("failed to marshal subscribe message")
		return err
	}
	return conn.Write(ctx, websocket.MessageText, msgJSON)
}
