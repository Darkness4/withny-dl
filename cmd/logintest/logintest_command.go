// Package logintest provide a command for testing the login.
package logintest

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/erikgeiser/promptkit/textinput"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

// Command is the command for logging in and testing the login.
var Command = &cli.Command{
	Name:  "login-test",
	Usage: "Test the login.",
	Action: func(cCtx *cli.Context) error {
		ctx, cancel := context.WithCancel(cCtx.Context)

		// Trap cleanup
		cleanChan := make(chan os.Signal, 1)
		signal.Notify(cleanChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-cleanChan
			cancel()
		}()

		jar, err := cookiejar.New(&cookiejar.Options{})
		if err != nil {
			log.Panic().Err(err).Msg("failed to create cookie jar")
		}
		hclient := &http.Client{Jar: jar, Timeout: time.Minute}

		usernameInput := textinput.New("Username/Email:")
		username, err := usernameInput.RunPrompt()
		if err != nil {
			return err
		}

		passwordInput := textinput.New("Password:")
		passwordInput.Hidden = true
		password, err := passwordInput.RunPrompt()
		if err != nil {
			return err
		}

		client := api.NewClient(hclient, &secret.Static{
			SavedCredentials: api.SavedCredentials{
				Username: username,
				Password: password,
			},
		}, secret.NewTmpCache())
		if err := client.Login(ctx); err != nil {
			log.Err(err).
				Msg("failed to login to withny")
			return err
		}

		log.Info().Msg("Login successful")
		return nil
	},
}
