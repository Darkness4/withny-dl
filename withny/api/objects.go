package api

import (
	"encoding/json"
	"time"
)

// MetaData is the metadata of the stream.
type MetaData struct {
	User   GetUserResponse
	Stream GetStreamsResponseElement
}

// LoginResponse is the response of the login request.
type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
}

// GetUserResponse is the response of the get user request.
type GetUserResponse struct {
	ID                            json.Number `json:"id"`
	UUID                          string      `json:"uuid"`
	Username                      string      `json:"username"`
	Name                          string      `json:"name"`
	ProfileText                   string      `json:"profileText"`
	ProfileImageURL               string      `json:"profileImageUrl"`
	HeaderImageURL                string      `json:"headerImageUrl"`
	Cast                          Cast        `json:"cast"`
	CreateRoomNotificationEnabled bool        `json:"createRoomNotificationEnabled"`
	ProfileTextPlain              string      `json:"profileTextPlain"`
}

// GetStreamsResponse is the response of the get streams request.
type GetStreamsResponse []GetStreamsResponseElement

// GetStreamsResponseElement is the element of the get streams response.
type GetStreamsResponseElement struct {
	UUID            string      `json:"uuid"`
	Title           string      `json:"title"`
	About           string      `json:"about"`
	ThumbnailURL    string      `json:"thumbnailUrl"`
	BillingMode     string      `json:"billingMode"`
	Price           json.Number `json:"price"`
	StreamingMethod string      `json:"streamingMethod"`
	StartedAt       time.Time   `json:"startedAt"`
	ClosedAt        any         `json:"closedAt"`
	DeviceID        json.Number `json:"deviceId"`
	Cast            Cast        `json:"cast"`
	HasTicket       bool        `json:"hasTicket"`
}

// Cast is the cast of the user.
type Cast struct {
	ID                      json.Number              `json:"id"`
	UUID                    string                   `json:"uuid"`
	Coupon                  string                   `json:"coupon"`
	ProfileImageURL         string                   `json:"profileImageUrl"`
	HeaderImageURL          string                   `json:"headerImageUrl"`
	IsFavorite              bool                     `json:"isFavorite"`
	CastSocialMediaAccounts []CastSocialMediaAccount `json:"castSocialMediaAccounts"`
	AgencySecret            AgencySecret             `json:"agencySecret"`
}

// CastSocialMediaAccount is the social media account of the cast.
type CastSocialMediaAccount struct {
	Platform    string `json:"platform"`
	ChannelName string `json:"username"`
}

// AgencySecret is the agency secret of the cast.
type AgencySecret struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	ChannelName string `json:"username"`
	Name        string `json:"name"`
}

// WSCommentResponse is the response of the WebSocket GraphQL Comments API.
type WSCommentResponse struct {
	Data struct {
		OnPostComment Comment `json:"onPostComment"`
	} `json:"data"`
}

// Comment is the comment of the stream.
type Comment struct {
	StreamUUID   string      `json:"streamUUID"`
	CommentUUID  string      `json:"commentUUID"`
	UserUUID     string      `json:"userUUID"`
	Username     string      `json:"username"`
	Name         string      `json:"name"`
	ContentType  string      `json:"contentType"`
	Content      string      `json:"content"`
	TipAmount    json.Number `json:"tipAmount"`
	ItemID       string      `json:"itemID"`
	ItemName     string      `json:"itemName"`
	ItemURI      string      `json:"itemURI"`
	AnimationURI string      `json:"animationURI"`
	ItemPower    json.Number `json:"itemPower"`
	ItemLifetime json.Number `json:"itemLifetime"`
	CreatedAt    *string     `json:"createdAt"`
	UpdatedAt    *string     `json:"updatedAt"`
	DeletedAt    *string     `json:"deletedAt"`
}

// ErrorResponse is the error response of the API.
type ErrorResponse struct {
	Message string      `json:"message"`
	Status  json.Number `json:"status"`
}
