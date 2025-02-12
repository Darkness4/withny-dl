// Package hls provides functions to download HLS streams.
package hls

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Darkness4/withny-dl/telemetry/metrics"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

const tracerName = "hls"

var (
	timeZero = time.Unix(0, 0)
	// ErrHLSForbidden is returned when the HLS download is stopped with a forbidden error.
	ErrHLSForbidden = errors.New("hls download stopped with forbidden error")
	// ErrStreamEnded is returned when the HLS stream has ended.
	ErrStreamEnded = errors.New("stream ended")
)

// HTTPError represents an HTTP error.
type HTTPError struct {
	Status int
	Body   string
	Method string
	URL    string
}

// Error returns the error message.
func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP error %s %s, code=%d, body=%s", e.Method, e.URL, e.Status, e.Body)
}

// DownloaderOption is a function that configures a Downloader.
type DownloaderOption func(*Downloader)

// WithPacketLossMax sets the maximum number of packets that can be lost.
func WithPacketLossMax(packetLossMax int) DownloaderOption {
	return func(d *Downloader) {
		d.packetLossMax = packetLossMax
	}
}

// WithFragmentRetries sets the number of retries for each fragment.
func WithFragmentRetries(fragmentRetries int) DownloaderOption {
	return func(d *Downloader) {
		d.fragmentRetries = fragmentRetries
	}
}

// WithPlaylistRetries sets the number of retries for the playlist.
func WithPlaylistRetries(playlistRetries int) DownloaderOption {
	return func(d *Downloader) {
		d.playlistRetries = playlistRetries
	}
}

// WithLogger sets the logger for the Downloader.
func WithLogger(log *zerolog.Logger) DownloaderOption {
	return func(d *Downloader) {
		d.log = log
	}
}

// Downloader is used to download HLS streams.
type Downloader struct {
	*api.Client
	packetLossMax   int
	fragmentRetries int
	playlistRetries int

	log *zerolog.Logger
	url string

	// ready is used to notify that the downloader is running.
	// This is to avoid stressing the users with warning logs.
	ready bool
}

// NewDownloader creates a new HLS downloader.
func NewDownloader(
	client *api.Client,
	url string,
	opts ...DownloaderOption,
) *Downloader {
	defaultLog := zerolog.Nop()
	d := Downloader{
		Client: client,
		url:    url,
		log:    &defaultLog,
	}

	for _, opt := range opts {
		opt(&d)
	}

	return &d
}

// GetFragmentURLs fetches the fragment URLs from the HLS manifest.
func (hls *Downloader) GetFragmentURLs(ctx context.Context) ([]Fragment, error) {
	var respBody io.ReadCloser
	var lastHTTPError HTTPError
	var count int
	for count = 0; count <= hls.playlistRetries; count++ {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		req, err := hls.NewAuthRequestWithContext(ctx, "GET", hls.url, nil)
		if err != nil {
			panic(err)
		}
		req.Header.Set(
			"Accept",
			"application/x-mpegURL, application/vnd.apple.mpegurl, application/json, text/plain",
		)
		req.Header.Set("Referer", "https://www.withny.fun/")
		req.Header.Set("Origin", "https://www.withny.fun")

		resp, err := hls.Client.Do(req)
		if err != nil {
			hls.log.Err(err).Msg("failed to fetch fragment URLs")
			return []Fragment{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			url, _ := url.Parse(hls.url)

			if resp.StatusCode == 403 {
				hls.log.Error().
					Str("url", url.String()).
					Int("response.status", resp.StatusCode).
					Str("response.body", string(body)).
					Str("method", "GET").
					Msg("http error")
				metrics.Downloads.Errors.Add(ctx, 1)
				return []Fragment{}, ErrHLSForbidden
			} else if resp.StatusCode == 404 {
				hls.log.Warn().
					Str("url", url.String()).
					Int("response.status", resp.StatusCode).
					Str("response.body", string(body)).
					Str("method", "GET").
					Msg("stream is no more available")
				return []Fragment{}, ErrStreamEnded
			} else if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				lastHTTPError = HTTPError{
					Status: resp.StatusCode,
					Body:   string(body),
					Method: "GET",
					URL:    url.String(),
				}
				hls.log.Warn().
					Str("url", lastHTTPError.URL).
					Int("response.status", lastHTTPError.Status).
					Str("response.body", lastHTTPError.Body).
					Str("method", lastHTTPError.Method).
					Int("count", count).
					Int("playlistRetries", hls.playlistRetries).
					Msg("http error, retrying")
				continue
			}

			hls.log.Error().
				Str("url", url.String()).
				Int("response.status", resp.StatusCode).
				Str("response.body", string(body)).
				Str("method", "GET").
				Msg("http error")
			metrics.Downloads.Errors.Add(ctx, 1)
			return []Fragment{}, HTTPError{
				Status: resp.StatusCode,
				Body:   string(body),
				Method: "GET",
				URL:    url.String(),
			}
		}

		respBody = resp.Body
		break
	}
	if count > hls.playlistRetries {
		hls.log.Error().
			Str("url", lastHTTPError.URL).
			Int("response.status", lastHTTPError.Status).
			Str("response.body", lastHTTPError.Body).
			Str("method", lastHTTPError.Method).
			Int("playlistRetries", hls.playlistRetries).
			Msg("giving up after too many http error")
		metrics.Downloads.Errors.Add(ctx, 1)
		return []Fragment{}, lastHTTPError
	}
	defer respBody.Close()

	scanner := bufio.NewScanner(respBody)
	fragments := make([]Fragment, 0, 10)
	exists := make(map[string]bool) // Avoid duplicates

	// URLs are supposedly sorted.
	var currentFragment Fragment
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, "#EXT-X-PROGRAM-DATE-TIME"):
			ts := strings.TrimPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:")
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				hls.log.Warn().
					Err(err).
					Str("ts", ts).
					Msg("failed to parse time, using now")
				t = time.Now()
			}
			currentFragment.Time = t
		case strings.HasPrefix(line, "https://") && !exists[line]:
			_, err := url.Parse(line)
			if err != nil {
				hls.log.Warn().
					Err(err).
					Msg("m3u8 returned a bad url, skipping that line")
				continue
			}
			currentFragment.URL = line
			fragments = append(fragments, Fragment{
				URL:  currentFragment.URL,
				Time: currentFragment.Time,
			})
			exists[line] = true
		}
	}

	if !hls.ready {
		hls.ready = true
		hls.log.Info().Msg("downloading")
	}
	return fragments, nil
}

// fillQueue continuously fetches fragments url until stream end
func (hls *Downloader) fillQueue(
	ctx context.Context,
	fragChan chan<- Fragment,
) (err error) {
	hls.log.Debug().Msg("started to fill queue")
	ctx, span := otel.Tracer(tracerName).Start(ctx, "hls.fillQueue")
	defer span.End()

	// Used for termination
	lastFragmentReceivedTimestamp := time.Now()

	var (
		lastFragmentName    string
		lastFragmentTime    time.Time
		useTimeBasedSorting = true
	)

	// Create a new ticker to log every 10 second
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	errorCount := 0

	for {
		select {
		case <-ticker.C:
			hls.log.Debug().Msg("still downloading")
		default:
			// Do nothing if the ticker hasn't ticked yet
		}

		fragments, err := hls.GetFragmentURLs(ctx)
		if err != nil {
			span.RecordError(err)

			// fillQueue will exits here because of a 404
			if errors.Is(err, ErrStreamEnded) {
				hls.log.Info().Msg("stream has ended")
				return io.EOF
			}

			// Failed to fetch playlist in time
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ECONNRESET) {
				errorCount++
				hls.log.Err(err).
					Int("error.count", errorCount).
					Int("error.max", hls.packetLossMax).
					Msg("GetFragmentURLs failed, retrying")
				metrics.Downloads.Errors.Add(ctx, 1)

				// Ignore the error if tolerated
				if errorCount <= hls.packetLossMax {
					time.Sleep(time.Second)
					continue
				}
			}

			hls.log.Err(err).Msg("GetFragmentURLs failed")

			// It can also exit here on context cancelled
			return err
		}

		newIdx := 0
		// Find the last fragment url to resume download
		if lastFragmentName != "" &&
			((useTimeBasedSorting && !lastFragmentTime.Equal(timeZero)) || !useTimeBasedSorting) {
			for i, u := range fragments {
				fragmentName := filepath.Base(u.URL)
				var fragmentTime time.Time
				if useTimeBasedSorting {
					if u.Time.Equal(timeZero) {
						hls.log.Warn().Msg("fragment time is zero, use name based sorting")
						useTimeBasedSorting = false
					} else {
						fragmentTime = u.Time
					}
				}

				if lastFragmentName == fragmentName &&
					((useTimeBasedSorting && lastFragmentTime.Compare(fragmentTime) >= 0) || !useTimeBasedSorting) {
					newIdx = i + 1
				}
			}
		}

		nNew := len(fragments) - newIdx
		if nNew > 0 {
			lastFragmentReceivedTimestamp = time.Now()
			hls.log.Trace().Any("fragments", fragments[newIdx:]).Msg("found new fragments")
		}

		for _, f := range fragments[newIdx:] {
			lastFragmentName = filepath.Base(f.URL)
			if useTimeBasedSorting {
				lastFragmentTime = f.Time
			}
			fragChan <- f
		}

		// fillQueue will also exit here if the stream has ended (and do not send any fragment)
		if time.Since(lastFragmentReceivedTimestamp) > 5*time.Minute {
			hls.log.Warn().
				Time("lastTime", lastFragmentReceivedTimestamp).
				Msg("timeout receiving new fragments, abort")
			return io.EOF
		}

		time.Sleep(time.Second)
	}
}

func (hls *Downloader) download(
	ctx context.Context,
	w io.Writer,
	url string,
) error {
	var respBody io.ReadCloser
	var lastHTTPError HTTPError
	var count int
	for count = 0; count <= hls.fragmentRetries; count++ {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		req, err := hls.NewAuthRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			panic(err)
		}
		req.Header.Set("Referer", "https://www.withny.fun/")
		req.Header.Set("Origin", "https://www.withny.fun")
		resp, err := hls.Client.Do(req)
		if err != nil {
			hls.log.Err(err).Msg("failed to download fragment")
			return err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 403 {
				hls.log.Error().
					Int("response.status", resp.StatusCode).
					Str("response.body", string(body)).
					Str("url", url).
					Str("method", "GET").
					Msg("http error")
				metrics.Downloads.Errors.Add(ctx, 1)
				return ErrHLSForbidden
			} else if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				lastHTTPError = HTTPError{
					Body:   string(body),
					Status: resp.StatusCode,
					Method: "GET",
					URL:    url,
				}
				hls.log.Warn().
					Str("url", lastHTTPError.URL).
					Int("response.status", lastHTTPError.Status).
					Str("response.body", lastHTTPError.Body).
					Str("method", lastHTTPError.Method).
					Int("count", count).
					Int("fragmentRetries", hls.fragmentRetries).
					Msg("http error, retrying")
				continue
			}

			hls.log.Error().
				Str("url", url).
				Int("response.status", resp.StatusCode).
				Str("response.body", string(body)).
				Str("method", "GET").
				Msg("http error")
			metrics.Downloads.Errors.Add(ctx, 1)
			return HTTPError{
				Body:   string(body),
				Status: resp.StatusCode,
				Method: "GET",
				URL:    url,
			}
		}

		respBody = resp.Body
		break
	}
	if count > hls.fragmentRetries {
		hls.log.Error().
			Str("url", lastHTTPError.URL).
			Int("response.status", lastHTTPError.Status).
			Str("response.body", lastHTTPError.Body).
			Str("method", lastHTTPError.Method).
			Int("fragmentRetries", hls.fragmentRetries).
			Msg("giving up after too many http error")
		metrics.Downloads.Errors.Add(ctx, 1)
		return lastHTTPError
	}
	defer respBody.Close()
	_, err := io.Copy(w, respBody)
	return err
}

// Fragment represents a fragment of the HLS stream.
type Fragment struct {
	URL  string
	Time time.Time
}

// Read reads the HLS stream and sends the data to the writer.
//
// Read runs two threads:
//
//  1. A goroutine will continuously fetch the fragment URLs and send them to the urlsChan.
//  2. The main thread will download the fragments and write them to the writer.
//
// The function will return when the context is canceled or when the stream ends.
func (hls *Downloader) Read(
	ctx context.Context,
	writer io.Writer,
) (err error) {
	hls.log.Debug().Msg("started to read stream")
	ctx, span := otel.Tracer(tracerName).Start(ctx, "hls.Read")
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)

	errChan := make(chan error) // Blocking channel is used to wait for fillQueue to finish.
	defer close(errChan)

	fragChan := make(chan Fragment, 10)
	defer close(fragChan)

	go func() {
		err := hls.fillQueue(ctx, fragChan)
		errChan <- err
	}()

	errorCount := 0

	for {
		select {
		case frag := <-fragChan:
			err := hls.download(ctx, writer, frag.URL)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					hls.log.Info().Msg("skip fragment download because of context canceled")
					continue // Continue to wait for fillQueue to finish
				}
				hls.log.Err(err).Msg("failed to download fragment")
				span.RecordError(err)
				if err == ErrHLSForbidden {
					hls.log.Err(err).Msg("stream was interrupted")
					cancel()
					continue // Continue to wait for fillQueue to finish
				}
				errorCount++
				hls.log.Error().
					Int("error.count", errorCount).
					Int("error.max", hls.packetLossMax).
					Err(err).
					Msg("a packet failed to be downloaded, skipping")
				metrics.Downloads.Errors.Add(ctx, 1)
				if errorCount <= hls.packetLossMax {
					continue
				}
				cancel()
				continue // Continue to wait for fillQueue to finish
			}

		// fillQueue will exit here if the stream has ended or context is canceled.
		case err := <-errChan:
			defer cancel()
			if err == nil {
				hls.log.Panic().Msg("didn't expect a nil error")
			}

			if err == io.EOF {
				hls.log.Info().Msg("hls downloader exited with success")
			} else if errors.Is(err, context.Canceled) {
				hls.log.Info().Msg("hls downloader canceled")
			} else {
				hls.log.Err(err).Msg("hls downloader exited with error")
			}

			return err
		}
	}
}

// Probe checks if the stream is ready to be downloaded.
func (hls *Downloader) Probe(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := hls.NewAuthRequestWithContext(ctx, "GET", hls.url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set(
		"Accept",
		"application/x-mpegURL, application/vnd.apple.mpegurl, application/json, text/plain",
	)
	req.Header.Set("Referer", "https://www.withny.fun/")
	req.Header.Set("Origin", "https://www.withny.fun")

	resp, err := hls.Client.Do(req)
	if err != nil {
		hls.log.Err(err).Msg("failed to probe stream")
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)

		switch resp.StatusCode {
		case 404:
			hls.log.Warn().
				Str("url", hls.url).
				Int("response.status", resp.StatusCode).
				Str("response.body", string(body)).
				Str("method", "GET").
				Msg("stream not ready")
			return false, nil
		default:
			hls.log.Error().
				Str("url", hls.url).
				Int("response.status", resp.StatusCode).
				Str("response.body", string(body)).
				Str("method", "GET").
				Msg("http error")
			return false, fmt.Errorf(
				"http error: url=%s, status=%d, method=GET",
				hls.url,
				resp.StatusCode,
			)
		}
	}

	return true, nil
}
