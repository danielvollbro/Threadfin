package src

import (
	"compress/gzip"
	"io"
	"os"
)

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
