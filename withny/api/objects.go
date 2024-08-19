package api

import (
	"time"
)

// Metadata is the metadata of the stream.
type Metadata struct {
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
	ID                            int64  `json:"id"`
	UUID                          string `json:"uuid"`
	Username                      string `json:"username"`
	Name                          string `json:"name"`
	ProfileText                   string `json:"profileText"`
	ProfileImageURL               string `json:"profileImageUrl"`
	HeaderImageURL                string `json:"headerImageUrl"`
	Cast                          Cast   `json:"cast"`
	CreateRoomNotificationEnabled bool   `json:"createRoomNotificationEnabled"`
	ProfileTextPlain              string `json:"profileTextPlain"`
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
	Price           int64       `json:"price"`
	StreamingMethod string      `json:"streamingMethod"`
	StartedAt       time.Time   `json:"startedAt"`
	ClosedAt        interface{} `json:"closedAt"`
	DeviceID        int64       `json:"deviceId"`
	Cast            Cast        `json:"cast"`
	HasTicket       bool        `json:"hasTicket"`
}

// Cast is the cast of the user.
type Cast struct {
	ID                      int64                    `json:"id"`
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
