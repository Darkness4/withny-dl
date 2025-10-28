// Package graphql defines the GraphQL messages.
package graphql

import (
	"github.com/google/uuid"
)

var (
	// ConnectionInit is the message to initialize the connection. (client -> server)
	ConnectionInit = map[string]any{
		"type": "connection_init",
	}
)

// ConnectionAckMessage is the message to acknowledge the connection. (server -> client)
type ConnectionAckMessage struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// SubscribeMessagePayload is the payload of the subscribe message.
type SubscribeMessagePayload struct {
	Data       string         `json:"data"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Query is the GraphQL query.
//
// This must be serialized to JSON and put in the Data field of the payload.
type Query struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// BuildSubscribeMessage builds a subscribe message.
func BuildSubscribeMessage(payload SubscribeMessagePayload) map[string]any {
	uuid := uuid.New().String()
	return map[string]any{
		"type":    "start",
		"id":      uuid,
		"payload": payload,
	}
}
