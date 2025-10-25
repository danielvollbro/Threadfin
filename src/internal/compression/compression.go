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

func ExtractZIP(archive, target string) (err error) {

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
