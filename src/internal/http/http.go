package http

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/storage"
	"time"
)

func DownloadFile(providerURL string, proxyUrl string) (filename string, body []byte, err error) {
	_, err = url.ParseRequestURI(providerURL)
	if err != nil {
		return
	}

	// Derive a timeout: prefer configured buffer timeout if provided, else default to 30s
	requestTimeout := 30 * time.Second
	if config.Settings.BufferTimeout > 0 {
		requestTimeout = time.Duration(config.Settings.BufferTimeout*1000) * time.Millisecond
	}

	httpClient := &http.Client{Timeout: requestTimeout}

	if proxyUrl != "" {
		proxyURL, err := url.Parse(proxyUrl)
		if err != nil {
			return "", nil, err
		}

		httpClient = &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
	}

	req, err := http.NewRequest("GET", providerURL, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", config.Settings.UserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		err = resp.Body.Close()
	}()
	if err != nil {
		return
	}

	resp.Header.Set("User-Agent", config.Settings.UserAgent)

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("%d: %s %s", resp.StatusCode, providerURL, http.StatusText(resp.StatusCode))
		return
	}

	// Get filename from the header
	var index = strings.Index(resp.Header.Get("Content-Disposition"), "filename")

	if index > -1 {
		var headerFilename = resp.Header.Get("Content-Disposition")[index:]
		var value = strings.Split(headerFilename, `=`)
		var f = strings.ReplaceAll(value[1], `"`, "")
		f = strings.ReplaceAll(f, `;`, "")
		filename = f
		cli.ShowInfo("Header filename:" + filename)
	} else {
		var cleanFilename = strings.SplitN(storage.GetFilenameFromPath(providerURL), "?", 2)
		filename = cleanFilename[0]
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}
