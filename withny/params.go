package withny

import (
	"encoding/json"
	"maps"
	"time"

	"github.com/Darkness4/withny-dl/withny/api"
)

// Params represents the parameters for the download.
type Params struct {
	QualityConstraint      api.PlaylistConstraint `yaml:"quality,omitempty"`
	PacketLossMax          int                    `yaml:"packetLossMax,omitempty"`
	FragmentRetries        int                    `yaml:"fragmentRetries,omitempty"`
	PlaylistRetries        int                    `yaml:"playlistRetries,omitempty"`
	OutFormat              string                 `yaml:"outFormat,omitempty"`
	WriteChat              bool                   `yaml:"writeChat,omitempty"`
	WriteMetaDataJSON      bool                   `yaml:"writeMetaDataJson,omitempty"`
	WriteThumbnail         bool                   `yaml:"writeThumbnail,omitempty"`
	WaitPollInterval       time.Duration          `yaml:"waitPollInterval,omitempty"`
	Remux                  bool                   `yaml:"remux,omitempty"`
	RemuxFormat            string                 `yaml:"remuxFormat,omitempty"`
	Concat                 bool                   `yaml:"concat,omitempty"`
	KeepIntermediates      bool                   `yaml:"keepIntermediates,omitempty"`
	ScanDirectory          string                 `yaml:"scanDirectory,omitempty"`
	EligibleForCleaningAge time.Duration          `yaml:"eligibleForCleaningAge,omitempty"`
	DeleteCorrupted        bool                   `yaml:"deleteCorrupted,omitempty"`
	ExtractAudio           bool                   `yaml:"extractAudio,omitempty"`
	PassCode               string                 `yaml:"passCode,omitempty"`
	Labels                 map[string]string      `yaml:"labels,omitempty"`
	Ignore                 []string               `yaml:"ignore,omitempty"`
}

func (p *Params) String() string {
	out, _ := json.MarshalIndent(p, "", "  ")
	return string(out)
}

// OptionalParams represents the optional parameters for the download.
type OptionalParams struct {
	QualityConstraint      *api.PlaylistConstraint `yaml:"quality,omitempty"`
	PacketLossMax          *int                    `yaml:"packetLossMax,omitempty"`
	FragmentRetries        *int                    `yaml:"fragmentRetries,omitempty"`
	PlaylistRetries        *int                    `yaml:"playlistRetries,omitempty"`
	OutFormat              *string                 `yaml:"outFormat,omitempty"`
	WriteChat              *bool                   `yaml:"writeChat,omitempty"`
	WriteMetaDataJSON      *bool                   `yaml:"writeMetaDataJson,omitempty"`
	WriteThumbnail         *bool                   `yaml:"writeThumbnail,omitempty"`
	WaitPollInterval       *time.Duration          `yaml:"waitPollInterval,omitempty"`
	Remux                  *bool                   `yaml:"remux,omitempty"`
	RemuxFormat            *string                 `yaml:"remuxFormat,omitempty"`
	Concat                 *bool                   `yaml:"concat,omitempty"`
	KeepIntermediates      *bool                   `yaml:"keepIntermediates,omitempty"`
	ScanDirectory          *string                 `yaml:"scanDirectory,omitempty"`
	EligibleForCleaningAge *time.Duration          `yaml:"eligibleForCleaningAge,omitempty"`
	DeleteCorrupted        *bool                   `yaml:"deleteCorrupted,omitempty"`
	ExtractAudio           *bool                   `yaml:"extractAudio,omitempty"`
	PassCode               *string                 `yaml:"passCode,omitempty"`
	Labels                 map[string]string       `yaml:"labels,omitempty"`
	Ignore                 []string                `yaml:"ignore,omitempty"`
}

// DefaultParams is the default set of parameters.
var DefaultParams = Params{
	QualityConstraint:      api.PlaylistConstraint{},
	PacketLossMax:          20,
	FragmentRetries:        10,
	PlaylistRetries:        10,
	OutFormat:              "{{ .Date }} {{ .Title }} ({{ .ChannelName }}).{{ .Ext }}",
	WriteChat:              false,
	WriteMetaDataJSON:      false,
	WriteThumbnail:         false,
	WaitPollInterval:       10 * time.Second,
	Remux:                  true,
	RemuxFormat:            "mp4",
	Concat:                 true,
	KeepIntermediates:      false,
	ScanDirectory:          "",
	EligibleForCleaningAge: 48 * time.Hour,
	DeleteCorrupted:        true,
	ExtractAudio:           false,
	PassCode:               "",
	Labels:                 nil,
	Ignore:                 []string{},
}

// Override applies the values from the OptionalParams to the Params.
func (override *OptionalParams) Override(params *Params) {
	if override.QualityConstraint != nil {
		params.QualityConstraint = *override.QualityConstraint
	}
	if override.PacketLossMax != nil {
		params.PacketLossMax = *override.PacketLossMax
	}
	if override.FragmentRetries != nil {
		params.FragmentRetries = *override.FragmentRetries
	}
	if override.PlaylistRetries != nil {
		params.PlaylistRetries = *override.PlaylistRetries
	}
	if override.OutFormat != nil {
		params.OutFormat = *override.OutFormat
	}
	if override.WriteChat != nil {
		params.WriteChat = *override.WriteChat
	}
	if override.WriteMetaDataJSON != nil {
		params.WriteMetaDataJSON = *override.WriteMetaDataJSON
	}
	if override.WriteThumbnail != nil {
		params.WriteThumbnail = *override.WriteThumbnail
	}
	if override.WaitPollInterval != nil {
		params.WaitPollInterval = *override.WaitPollInterval
	}
	if override.Remux != nil {
		params.Remux = *override.Remux
	}
	if override.RemuxFormat != nil {
		params.RemuxFormat = *override.RemuxFormat
	}
	if override.Concat != nil {
		params.Concat = *override.Concat
	}
	if override.KeepIntermediates != nil {
		params.KeepIntermediates = *override.KeepIntermediates
	}
	if override.ScanDirectory != nil {
		params.ScanDirectory = *override.ScanDirectory
	}
	if override.EligibleForCleaningAge != nil {
		params.EligibleForCleaningAge = *override.EligibleForCleaningAge
	}
	if override.DeleteCorrupted != nil {
		params.DeleteCorrupted = *override.DeleteCorrupted
	}
	if override.ExtractAudio != nil {
		params.ExtractAudio = *override.ExtractAudio
	}
	if override.PassCode != nil {
		params.PassCode = *override.PassCode
	}
	if override.Labels != nil {
		if params.Labels == nil {
			params.Labels = make(map[string]string)
		}
		maps.Copy(params.Labels, override.Labels)
	}
	if override.Ignore != nil {
		params.Ignore = override.Ignore
	}
}

// Clone creates a deep copy of the Params struct.
func (p *Params) Clone() *Params {
	// Create a new Params struct with the same field values as the original
	clone := Params{
		QualityConstraint:      p.QualityConstraint,
		PacketLossMax:          p.PacketLossMax,
		FragmentRetries:        p.FragmentRetries,
		PlaylistRetries:        p.PlaylistRetries,
		OutFormat:              p.OutFormat,
		WriteChat:              p.WriteChat,
		WriteMetaDataJSON:      p.WriteMetaDataJSON,
		WriteThumbnail:         p.WriteThumbnail,
		WaitPollInterval:       p.WaitPollInterval,
		Remux:                  p.Remux,
		RemuxFormat:            p.RemuxFormat,
		Concat:                 p.Concat,
		KeepIntermediates:      p.KeepIntermediates,
		ScanDirectory:          p.ScanDirectory,
		EligibleForCleaningAge: p.EligibleForCleaningAge,
		DeleteCorrupted:        p.DeleteCorrupted,
		ExtractAudio:           p.ExtractAudio,
		PassCode:               p.PassCode,
		Ignore:                 make([]string, len(p.Ignore)),
	}

	// Clone the labels map if it exists
	if p.Labels != nil {
		clone.Labels = make(map[string]string)
		maps.Copy(clone.Labels, p.Labels)
	}

	// Clone the ignore slice
	copy(clone.Ignore, p.Ignore)

	return &clone
}
