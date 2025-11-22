package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"github.com/rs/zerolog/log"
)

// MustJSONEncode encodes the value to JSON or panic.
func MustJSONEncode(v any) string {
	var bb bytes.Buffer
	if err := json.NewEncoder(&bb).Encode(v); err != nil {
		panic(err.Error())
	}
	return bb.String()
}

// JSONDecodeAndPrintOnError decodes JSON from the reader and logs the raw JSON on error.
func JSONDecodeAndPrintOnError(r io.Reader, v any) error {
	var rawData bytes.Buffer
	tee := io.TeeReader(r, &rawData)

	decoder := json.NewDecoder(tee)

	err := decoder.Decode(v)
	if err != nil {
		log.Err(err).
			Str("parentCaller", getCaller()).
			Str("raw_message", rawData.String()).
			Msg("failed to decode JSON")
	}
	return err
}

func getCaller() string {
	// Skip 2 frames to get the caller of the function calling this function
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}
