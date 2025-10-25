package compression

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"threadfin/src/internal/config"
)

func ZipFiles(sourceFiles []string, target string) error {

	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		err = zipfile.Close()
	}()
	if err != nil {
		return err
	}

	archive := zip.NewWriter(zipfile)
	defer func() {
		err = archive.Close()
	}()
	if err != nil {
		return err
	}

	for _, source := range sourceFiles {

		info, err := os.Stat(source)
		if err != nil {
			return nil
		}

		var baseDir string
		if info.IsDir() {
			baseDir = filepath.Base(config.System.Folder.Data)
		}

		err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			if baseDir != "" {
				header.Name = filepath.Join(strings.TrimPrefix(path, config.System.Folder.Config))
			}

			if info.IsDir() {
				header.Name += string(os.PathSeparator)
			} else {
				header.Method = zip.Deflate
			}

			writer, err := archive.CreateHeader(header)
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() {
				err = file.Close()
			}()
			if err != nil {
				return err
			}

			_, err = io.Copy(writer, file)

			return err

		})
		if err != nil {
			return err
		}

	}

	return err
}
