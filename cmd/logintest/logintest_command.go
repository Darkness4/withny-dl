// Package logintest provide a command for testing the login.
package logintest

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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
	Usage: "Test the login and encode to base64.",
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

		client := api.NewClient(hclient, &secret.UserPasswordStatic{
			Email:    username,
			Password: password,
		})
		if err := client.Login(ctx); err != nil {
			log.Err(err).
				Msg("failed to login to withny")
			return err
		}

		log.Info().Msg("Login successful")

		usernameB64 := base64.StdEncoding.EncodeToString([]byte(username))
		passwordB64 := base64.StdEncoding.EncodeToString([]byte(password))

		f, err := createFileWithRename("credentials.txt")
		if err != nil {
			log.Err(err).Msg("failed to create credentials.txt")
			return err
		}
		defer f.Close()
		if err := f.Chmod(0600); err != nil {
			log.Err(err).Msg("failed to chmod credentials.txt")
			return err
		}
		if _, err := f.WriteString(usernameB64 + ":" + passwordB64); err != nil {
			log.Err(err).Msg("failed to write credentials.txt")
			return err
		}

		log.Info().Msg("Credentials written to credentials.txt")
		return nil
	},
}

// Create a file, renaming it if there's a conflict
func createFileWithRename(path string) (*os.File, error) {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	dir := filepath.Dir(path)
	name := base[:len(base)-len(ext)]

	// Attempt to create the file, renaming it if necessary
	for i := 0; ; i++ {
		newPath := filepath.Join(dir, name+ext)
		if i > 0 {
			newPath = filepath.Join(dir, name+"_"+strconv.Itoa(i)+ext)
		}

		// Attempt to open the file for creation
		file, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Successfully created the file
			return file, nil
		} else if os.IsExist(err) {
			// File already exists, continue renaming
			continue
		} else {
			// Some other error occurred
			return nil, err
		}
	}
}
