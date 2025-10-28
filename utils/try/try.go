// Package try provides a set of functions to retry a function with a delay.
//
// nolint: ireturn
package try

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"
)

// Do tries a function with a delay.
func Do(
	tries int,
	delay time.Duration,
	fn func() error,
) (err error) {
	if tries <= 0 {
		log.Panic().Int("tries", tries).Msg("tries is 0 or negative")
	}
	for try := range tries {
		err = fn()
		if err == nil {
			return nil
		}
		log.Warn().
			Str("parentCaller", getCaller()).
			Err(err).
			Int("try", try).
			Int("maxTries", tries).
			Msg("try failed")
		time.Sleep(delay)
	}
	log.Warn().Err(err).Msg("failed all tries")
	return err
}

// DoExponentialBackoff tries a function with exponential backoff.
func DoExponentialBackoff(
	tries int,
	delay time.Duration,
	multiplier time.Duration,
	maxBackoff time.Duration,
	fn func() error,
) (err error) {
	if tries <= 0 {
		log.Panic().Int("tries", tries).Msg("tries is 0 or negative")
	}
	for try := range tries {
		err = fn()
		if err == nil {
			return nil
		}
		log.Warn().
			Str("parentCaller", getCaller()).
			Err(err).
			Int("try", try).
			Int("maxTries", tries).
			Stringer("backoff", delay).
			Msg("try failed")
		time.Sleep(delay)
		delay = min(delay*multiplier, maxBackoff)
	}
	log.Warn().Err(err).Msg("failed all tries")
	return err
}

// DoWithResult tries a function and returns a result.
//
// To avoid any deadlock, the function will stop if the errors is context.Canceled.
func DoWithResult[T any](
	tries int,
	delay time.Duration,
	fn func() (T, error),
) (result T, err error) {
	if tries <= 0 {
		log.Panic().Int("tries", tries).Msg("tries is 0 or negative")
	}
	for try := range tries {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if errors.Is(err, context.Canceled) {
			return result, err
		}
		log.Warn().Str("parentCaller", getCaller()).Int("try", try).Err(err).Msg("try failed")
		time.Sleep(delay)
	}
	log.Warn().Err(err).Msg("failed all tries")
	return result, err
}

// DoExponentialBackoffWithResult performs an exponential backoff and return a result.
//
// To avoid any deadlock, the function will stop if the errors is context.Canceled.
func DoExponentialBackoffWithResult[T any](
	tries int,
	delay time.Duration,
	multiplier int,
	maxBackoff time.Duration,
	fn func() (T, error),
) (result T, err error) {
	if tries <= 0 {
		log.Panic().Int("tries", tries).Msg("tries is 0 or negative")
	}
	for try := range tries {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if errors.Is(err, context.Canceled) {
			return result, err
		}
		log.Warn().
			Str("parentCaller", getCaller()).
			Int("try", try).
			Int("maxTries", tries).
			Stringer("backoff", delay).
			Err(err).Msg(
			"try failed",
		)
		time.Sleep(delay)
		delay = min(delay*time.Duration(multiplier), maxBackoff)
	}
	log.Warn().Err(err).Msg("failed all tries")
	return result, err
}

func getCaller() string {
	// Skip 2 frames to get the caller of the function calling this function
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}
