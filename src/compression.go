package src

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"threadfin/src/internal/cli"
)

func extractGZIP(gzipBody []byte, fileSource string) (body []byte, err error) {

	var b = bytes.NewBuffer(gzipBody)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		// Keine gzip Datei
		body = gzipBody
		err = nil
		return
	}

	cli.ShowInfo("Extract gzip:" + fileSource)

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		body = gzipBody
		err = nil
		return
	}

	body = resB.Bytes()
	return
}

func compressGZIPFile(sourcePath, targetPath string) (err error) {
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() {
		err = in.Close()
	}()
	if err != nil {
		return err
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer func() {
		err = out.Close()
	}()
	if err != nil {
		return err
	}

	gw := gzip.NewWriter(out)
	defer func() {
		err = gw.Close()
	}()
	if err != nil {
		return err
	}

	_, err = io.Copy(gw, in)
	return err
}
