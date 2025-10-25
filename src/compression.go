package src

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"threadfin/src/internal/cli"
)

func extractZIP(archive, target string) (err error) {

	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {

		path := filepath.Join(target, file.Name)
		if file.FileInfo().IsDir() {
			err = os.MkdirAll(path, file.Mode())
			if err != nil {
				return err
			}

			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer func() {
			err = fileReader.Close()
		}()
		if err != nil {
			return err
		}

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer func() {
			err = targetFile.Close()
		}()
		if err != nil {
			return err
		}

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}

	}

	return
}

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
