// Package try provides a set of functions to retry a function with a delay.
//
// nolint: ireturn
package try

import (
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
	for try := 0; try < tries; try++ {
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
	for try := 0; try < tries; try++ {
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
		delay = delay * multiplier
		if delay > maxBackoff {
			delay = maxBackoff
		}
	}
	log.Warn().Err(err).Msg("failed all tries")
	return err
}

// DoWithResult tries a function and returns a result.
func DoWithResult[T any](
	tries int,
	delay time.Duration,
	fn func() (T, error),
) (result T, err error) {
	if tries <= 0 {
		log.Panic().Int("tries", tries).Msg("tries is 0 or negative")
	}
	for try := 0; try < tries; try++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		log.Warn().Str("parentCaller", getCaller()).Int("try", try).Err(err).Msg("try failed")
		time.Sleep(delay)
	}
	log.Warn().Err(err).Msg("failed all tries")
	return result, err
}

// DoExponentialBackoffWithResult performs an exponential backoff and return a result.
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
	for try := 0; try < tries; try++ {
		result, err = fn()
		if err == nil {
			return result, nil
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
		delay = delay * time.Duration(multiplier)
		if delay > maxBackoff {
			delay = maxBackoff
		}
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
