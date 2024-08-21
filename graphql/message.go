// Package graphql defines the GraphQL messages.
package graphql

import (
	"github.com/google/uuid"
)

var (
	// ConnectionInit is the message to initialize the connection. (client -> server)
	ConnectionInit = map[string]interface{}{
		"type": "connection_init",
	}
)

// ConnectionAckMessage is the message to acknowledge the connection. (server -> client)
type ConnectionAckMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// SubscribeMessagePayload is the payload of the subscribe message.
type SubscribeMessagePayload struct {
	Data       string                 `json:"data"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Query is the GraphQL query.
//
// This must be serialized to JSON and put in the Data field of the payload.
type Query struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// BuildSubscribeMessage builds a subscribe message.
func BuildSubscribeMessage(payload SubscribeMessagePayload) map[string]interface{} {
	uuid := uuid.New().String()
	return map[string]interface{}{
		"type":    "start",
		"id":      uuid,
		"payload": payload,
	}
}
