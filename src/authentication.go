package src

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"threadfin/src/internal/authentication"
	"threadfin/src/internal/config"
)

func basicAuth(r *http.Request, level string) (username string, err error) {

	err = errors.New("user authentication failed")

	auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {
		return
	}

	payload, _ := base64.StdEncoding.DecodeString(auth[1])
	pair := strings.SplitN(string(payload), ":", 2)

	username = pair[0]
	var password = pair[1]

	token, err := authentication.UserAuthentication(username, password)

	if err != nil {
		return
	}

	err = checkAuthorizationLevel(token, level)

	return
}

func urlAuth(r *http.Request, requestType string) (err error) {
	var level, token string

	var username = r.URL.Query().Get("username")
	var password = r.URL.Query().Get("password")

	switch requestType {

	case "m3u":
		level = "authentication.m3u"
		if config.Settings.AuthenticationM3U {
			token, err = authentication.UserAuthentication(username, password)
			if err != nil {
				return
			}
			err = checkAuthorizationLevel(token, level)
		}

	case "xml":
		level = "authentication.xml"
		if config.Settings.AuthenticationXML {
			token, err = authentication.UserAuthentication(username, password)
			if err != nil {
				return
			}
			err = checkAuthorizationLevel(token, level)
		}

	}

	return
}

func checkAuthorizationLevel(token, level string) (err error) {

	var authenticationErr = func(err error) {
		if err != nil {
			return
		}
	}

	userID, err := authentication.GetUserID(token)
	authenticationErr(err)

	userData, err := authentication.ReadUserData(userID)
	authenticationErr(err)

	if len(userData) > 0 {

		if v, ok := userData[level].(bool); ok {

			if !v {
				err = errors.New("no authorization")
			}

		} else {
			userData[level] = false
			err = authentication.WriteUserData(userID, userData)
			authenticationErr(err)
			err = errors.New("no authorization")
		}

	} else {
		err = authentication.WriteUserData(userID, userData)
		authenticationErr(err)
		err = errors.New("no authorization")
	}

	return
}
