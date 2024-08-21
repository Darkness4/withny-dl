package graphql

import (
	"github.com/google/uuid"
)

var (
	ConnectionInit = map[string]interface{}{
		"type": "connection_init",
	}
)

type ConnectionAckMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type SubscribeMessagePayload struct {
	Data       string                 `json:"data"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

type Query struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func BuildSubscribeMessage(payload SubscribeMessagePayload) map[string]interface{} {
	uuid := uuid.New().String()
	return map[string]interface{}{
		"type":    "start",
		"id":      uuid,
		"payload": payload,
	}
}
