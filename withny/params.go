package withny

import (
	"encoding/json"
	"time"

	"github.com/Darkness4/withny-dl/withny/api"
)

// Params represents the parameters for the download.
type Params struct {
	QualityConstraint      api.PlaylistConstraint `yaml:"quality,omitempty"`
	PacketLossMax          int                    `yaml:"packetLossMax,omitempty"`
	OutFormat              string                 `yaml:"outFormat,omitempty"`
	WaitForLive            bool                   `yaml:"waitForLive,omitempty"`
	WaitPollInterval       time.Duration          `yaml:"waitPollInterval,omitempty"`
	Remux                  bool                   `yaml:"remux,omitempty"`
	RemuxFormat            string                 `yaml:"remuxFormat,omitempty"`
	Concat                 bool                   `yaml:"concat,omitempty"`
	KeepIntermediates      bool                   `yaml:"keepIntermediates,omitempty"`
	ScanDirectory          string                 `yaml:"scanDirectory,omitempty"`
	EligibleForCleaningAge time.Duration          `yaml:"eligibleForCleaningAge,omitempty"`
	DeleteCorrupted        bool                   `yaml:"deleteCorrupted,omitempty"`
	ExtractAudio           bool                   `yaml:"extractAudio,omitempty"`
	Labels                 map[string]string      `yaml:"labels,omitempty"`
}

func (p *Params) String() string {
	out, _ := json.MarshalIndent(p, "", "  ")
	return string(out)
}

// OptionalParams represents the optional parameters for the download.
type OptionalParams struct {
	QualityConstraint      *api.PlaylistConstraint `yaml:"quality,omitempty"`
	PacketLossMax          *int                    `yaml:"packetLossMax,omitempty"`
	OutFormat              *string                 `yaml:"outFormat,omitempty"`
	WaitForLive            *bool                   `yaml:"waitForLive,omitempty"`
	WaitPollInterval       *time.Duration          `yaml:"waitPollInterval,omitempty"`
	Remux                  *bool                   `yaml:"remux,omitempty"`
	RemuxFormat            *string                 `yaml:"remuxFormat,omitempty"`
	Concat                 *bool                   `yaml:"concat,omitempty"`
	KeepIntermediates      *bool                   `yaml:"keepIntermediates,omitempty"`
	ScanDirectory          *string                 `yaml:"scanDirectory,omitempty"`
	EligibleForCleaningAge *time.Duration          `yaml:"eligibleForCleaningAge,omitempty"`
	DeleteCorrupted        *bool                   `yaml:"deleteCorrupted,omitempty"`
	ExtractAudio           *bool                   `yaml:"extractAudio,omitempty"`
	Labels                 map[string]string       `yaml:"labels,omitempty"`
}

// DefaultParams is the default set of parameters.
var DefaultParams = Params{
	QualityConstraint:      api.PlaylistConstraint{},
	PacketLossMax:          20,
	OutFormat:              "{{ .Date }} {{ .Title }} ({{ .ChannelName }}).{{ .Ext }}",
	WaitForLive:            true,
	WaitPollInterval:       5 * time.Second,
	Remux:                  true,
	RemuxFormat:            "mp4",
	Concat:                 true,
	KeepIntermediates:      false,
	ScanDirectory:          "",
	EligibleForCleaningAge: 48 * time.Hour,
	DeleteCorrupted:        true,
	ExtractAudio:           false,
	Labels:                 nil,
}

// Override applies the values from the OptionalParams to the Params.
func (override *OptionalParams) Override(params *Params) {
	if override.QualityConstraint != nil {
		params.QualityConstraint = *override.QualityConstraint
	}
	if override.PacketLossMax != nil {
		params.PacketLossMax = *override.PacketLossMax
	}
	if override.OutFormat != nil {
		params.OutFormat = *override.OutFormat
	}
	if override.WaitForLive != nil {
		params.WaitForLive = *override.WaitForLive
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
	if override.Labels != nil {
		if params.Labels == nil {
			params.Labels = make(map[string]string)
		}
		for k, v := range override.Labels {
			params.Labels[k] = v
		}
	}
}

// Clone creates a deep copy of the Params struct.
func (p *Params) Clone() *Params {
	// Create a new Params struct with the same field values as the original
	clone := Params{
		QualityConstraint:      p.QualityConstraint,
		PacketLossMax:          p.PacketLossMax,
		OutFormat:              p.OutFormat,
		WaitForLive:            p.WaitForLive,
		WaitPollInterval:       p.WaitPollInterval,
		Remux:                  p.Remux,
		RemuxFormat:            p.RemuxFormat,
		Concat:                 p.Concat,
		KeepIntermediates:      p.KeepIntermediates,
		ScanDirectory:          p.ScanDirectory,
		EligibleForCleaningAge: p.EligibleForCleaningAge,
		DeleteCorrupted:        p.DeleteCorrupted,
		ExtractAudio:           p.ExtractAudio,
	}

	// Clone the labels map if it exists
	if p.Labels != nil {
		clone.Labels = make(map[string]string)
		for k, v := range p.Labels {
			clone.Labels[k] = v
		}
	}

	return &clone
}
