package up2date

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"threadfin/internal/compression"
	"threadfin/internal/storage"

	"github.com/kardianos/osext"
)

// DoUpdate : Update binary
func DoUpdate(fileType, filenameBIN string) (err error) {

	var url string
	switch fileType {
	case "bin":
		url = Updater.Response.UpdateBIN
	case "zip":
		url = Updater.Response.UpdateZIP
	}

	switch runtime.GOOS {
	case "windows":
		filenameBIN = filenameBIN + ".exe"
	}

	if len(url) > 0 {
		log.Println("["+strings.ToUpper(fileType)+"]", "New version ("+Updater.Name+"):", Updater.Response.Version)

		// Download new binary
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer func() {
			err = resp.Body.Close()
		}()
		if err != nil {
			return err
		}

		log.Println("["+strings.ToUpper(fileType)+"]", "Download new version...")

		if resp.StatusCode != http.StatusOK {
			log.Println("["+strings.ToUpper(fileType)+"]", "Download new version...OK")
			return fmt.Errorf("bad status: %s", resp.Status)
		}

		// Change binary filename to .filename
		binary, _ := osext.Executable()
		var filename = storage.GetFilenameFromPath(binary)
		var path = storage.GetPlatformPath(binary)
		var oldBinary = path + "_old_" + filename
		var newBinary = binary

		// ZIP
		var tmpFolder = path + "tmp"
		var tmpFile = tmpFolder + string(os.PathSeparator) + filenameBIN

		err = os.Rename(newBinary, oldBinary)
		if err != nil {
			return err
		}

		// Save the new binary with the old file name
		out, err := os.Create(binary)
		if err != nil {
			restorOldBinary(oldBinary, newBinary)
			return err
		}
		defer func() {
			err = out.Close()
		}()
		if err != nil {
			restorOldBinary(oldBinary, newBinary)
			return err
		}

		// Write the body to file

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			restorOldBinary(oldBinary, newBinary)
			return err
		}

		// Update as a ZIP file
		if fileType == "zip" {

			log.Println("["+strings.ToUpper(fileType)+"]", "Update file:", filenameBIN)
			log.Println("["+strings.ToUpper(fileType)+"]", "Unzip ZIP file...")
			err = compression.ExtractZIP(binary, tmpFolder)

			binary = newBinary

			if err != nil {

				log.Println("["+strings.ToUpper(fileType)+"]", "Unzip ZIP file...ERROR")

				restorOldBinary(oldBinary, newBinary)

				return err
			} else {

				log.Println("["+strings.ToUpper(fileType)+"]", "Unzip ZIP file...OK")
				log.Println("["+strings.ToUpper(fileType)+"]", "Copy binary file...")

				err = copyFile(tmpFile, binary)
				if err == nil {
					log.Println("["+strings.ToUpper(fileType)+"]", "Copy binary file...OK")
				} else {

					log.Println("["+strings.ToUpper(fileType)+"]", "Copy binary file...ERROR")
					restorOldBinary(oldBinary, newBinary)

					return err
				}

				err = os.RemoveAll(tmpFolder)
				if err != nil {
					log.Println("["+strings.ToUpper(fileType)+"]", "Remove temp folder...ERROR")
					restorOldBinary(oldBinary, newBinary)
					return err
				}
			}

		}

		// Set the permission
		err = os.Chmod(binary, 0755)
		if err != nil {
			restorOldBinary(oldBinary, newBinary)
			return err
		}

		// Close the new file !Windows
		err = out.Close()
		if err != nil {
			restorOldBinary(oldBinary, newBinary)
			return err
		}

		log.Println("["+strings.ToUpper(fileType)+"]", "Update Successful")

		// Restart binary (Windows)
		if runtime.GOOS == "windows" {

			bin, err := os.Executable()

			if err != nil {
				restorOldBinary(oldBinary, newBinary)
				return err
			}

			var pid = os.Getpid()
			var process, _ = os.FindProcess(pid)

			if proc, err := start(bin); err == nil {

				err = os.RemoveAll(oldBinary)
				if err != nil {
					restorOldBinary(oldBinary, newBinary)
					log.Fatal(err)
					return err
				}

				err = process.Kill()
				if err != nil {
					log.Fatal(err)
					return err
				}

				_, err = proc.Wait()
				if err != nil {
					log.Fatal(err)
					return err
				}

			} else {
				restorOldBinary(oldBinary, newBinary)
			}

		} else {

			// Restart binary (Linux and UNIX)
			file, _ := osext.Executable()
			err = os.RemoveAll(oldBinary)
			if err != nil {
				restorOldBinary(oldBinary, newBinary)
				log.Fatal(err)
				return err
			}

			err = syscall.Exec(file, os.Args, os.Environ())
			if err != nil {
				restorOldBinary(oldBinary, newBinary)
				log.Fatal(err)
				return err
			}

		}

	}

	return
}

func start(args ...string) (p *os.Process, err error) {

	if args[0], err = exec.LookPath(args[0]); err == nil {
		var procAttr os.ProcAttr
		procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
		p, err := os.StartProcess(args[0], args, &procAttr)

		if err == nil {
			return p, nil
		}

	}

	return nil, err
}

func restorOldBinary(oldBinary, newBinary string) {
	err := os.RemoveAll(newBinary)
	if err != nil {
		log.Println("[UPDATE]", "Remove new binary...ERROR")
	}

	err = os.Rename(oldBinary, newBinary)
	if err != nil {
		log.Println("[UPDATE]", "Restor old binary...ERROR")
	}
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() {
		err = in.Close()
	}()
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		err = out.Close()
	}()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
