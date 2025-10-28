// Package socketio provides the socket.io handlers.
package socketio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidVersion is returned when the message version is invalid.
	ErrInvalidVersion = errors.New("unhandled message version")
	// ErrInvalidType is returned when the message type is invalid.
	ErrInvalidType = errors.New("unhandled message type")
	// ErrInvalidPacket is returned when the packet is invalid.
	ErrInvalidPacket = errors.New("invalid packet")
)

// MessageType is a socket.io message type.
type MessageType byte

const (
	// MessageTypeConnect used during the connection to a namespace.
	MessageTypeConnect MessageType = iota
	// MessageTypeDisconnect used when disconnecting from a namespace.
	MessageTypeDisconnect
	// MessageTypeEvent used to send data to the other side.
	MessageTypeEvent
	// MessageTypeAck used to acknowledge an event.
	MessageTypeAck
	// MessageTypeConnectError used during the connection to a namespace.
	MessageTypeConnectError
	// MessageTypeBinaryEvent ysed to send binary data to the other side.
	MessageTypeBinaryEvent
	// MessageTypeBinaryAck ysed to acknowledge an event (the response includes binary data).
	MessageTypeBinaryAck
)

// MessageV4 is a socket.io v4 message.
type MessageV4 struct {
	Type        MessageType
	Attachments int
	Namespace   string
	ID          int
	Payload     []byte
}

// UnmarshalMessageType unmarshals a byte into a MessageType.
func UnmarshalMessageType(data byte) (MessageType, error) {
	switch data {
	case 0:
		return MessageTypeConnect, nil
	case 1:
		return MessageTypeDisconnect, nil
	case 2:
		return MessageTypeEvent, nil
	case 3:
		return MessageTypeAck, nil
	case 4:
		return MessageTypeConnectError, nil
	case 5:
		return MessageTypeBinaryEvent, nil
	case 6:
		return MessageTypeBinaryAck, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrInvalidType, data)
	}
}

// UnmarshalV4 unmarshals a packet into a MessageV4.
//
// Packet looks like: <packet type>[<# of binary attachments>-][<namespace>,][<acknowledgment id>][JSON-stringified payload without binary]
//
// Note that if attachments if greater than 0, the next packets will be pure binary
// data.
func UnmarshalV4(data []byte) (msg MessageV4, err error) {
	if len(data) < 2 {
		return msg, ErrInvalidPacket
	}

	if data[0]-48 != 4 {
		return msg, fmt.Errorf("%w: %d", ErrInvalidVersion, data[0]-48)
	}

	typ, err := UnmarshalMessageType(data[1] - 48)
	if err != nil {
		return msg, err
	}
	msg.Type = typ

	idx := 2

	// At this point we can encounter optional fields
	// Check if it's a int (which means, the number attachment)
	if idx < len(data) && data[idx] >= 48 && data[idx] <= 57 {
		// Parse the number until encountering an '-'
		msg.Attachments = 0
		for idx < len(data) && data[idx] != '-' {
			msg.Attachments = msg.Attachments*10 + int(data[idx]-'0')
			idx++
		}

		// idx is on the '-'
		if data[idx] == '-' {
			idx++
		}
	}

	// Check if it's a namespace (which begins with '/')
	if idx < len(data) && data[idx] == '/' {
		// Parse the namespace until encountering a ','
		msg.Namespace = ""
		for idx < len(data) && data[idx] != ',' {
			msg.Namespace += string(data[idx])
			idx++
		}

		// idx is on the ','
		if data[idx] == ',' {
			idx++
		}
	}

	// Check if it's a int (which means, the acknowledgement number)
	if idx < len(data) && data[idx] >= 48 && data[idx] <= 57 {
		// Parse the number until encountering a '{' or '['
		msg.ID = 0
		for idx < len(data) && data[idx] != ',' {
			msg.ID = msg.ID*10 + int(data[idx]-'0')
			idx++
		}
	}

	// Check if there is a payload (which begins either with '{' or '[')
	if idx < len(data) && data[idx] == '{' || data[idx] == '[' {
		msg.Payload = data[idx:]
	}

	return msg, nil
}
