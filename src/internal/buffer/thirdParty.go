package buffer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/client"
	"threadfin/src/internal/config"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"time"
)

// Buffer with FFMPEG and VLC
func ThirdParty(streamID int, playlistID string, useBackup bool, backupNumber int) {

	if p, ok := config.BufferInformation.Load(playlistID); ok {

		var playlist = p.(structs.Playlist)
		var debug, path, options, bufferType string
		var tmpSegment = 1
		var bufferSize = config.Settings.BufferSize * 1024
		var stream = playlist.Streams[streamID]
		var buf bytes.Buffer
		var fileSize = 0
		var streamStatus = make(chan bool)

		var tmpFolder = playlist.Streams[streamID].Folder
		var url = playlist.Streams[streamID].URL
		if useBackup {
			if backupNumber >= 1 && backupNumber <= 3 {
				switch backupNumber {
				case 1:
					if stream.BackupChannel1 != nil {
						url = stream.BackupChannel1.URL
						cli.ShowHighlight("START OF BACKUP 1 STREAM")
						cli.ShowInfo("Backup Channel 1 URL: " + url)
					}
				case 2:
					if stream.BackupChannel2 != nil {
						url = stream.BackupChannel2.URL
						cli.ShowHighlight("START OF BACKUP 2 STREAM")
						cli.ShowInfo("Backup Channel 2 URL: " + url)
					}
				case 3:
					if stream.BackupChannel3 != nil {
						url = stream.BackupChannel3.URL
						cli.ShowHighlight("START OF BACKUP 3 STREAM")
						cli.ShowInfo("Backup Channel 3 URL: " + url)
					}
				}
			}
		}

		stream.Status = false

		bufferType = strings.ToUpper(playlist.Buffer)

		switch playlist.Buffer {

		case "ffmpeg":

			if config.Settings.FFmpegForceHttp {
				url = strings.ReplaceAll(url, "https://", "http://")
				cli.ShowInfo("Forcing URL to HTTP for FFMPEG: " + url)
			}

			path = config.Settings.FFmpegPath
			options = config.Settings.FFmpegOptions

		case "vlc":
			path = config.Settings.VLCPath
			options = config.Settings.VLCOptions

		default:
			return
		}

		var addErrorToStream = func(err error) {
			if !useBackup || (useBackup && backupNumber >= 0 && backupNumber <= 3) {
				backupNumber = backupNumber + 1
				if stream.BackupChannel1 != nil || stream.BackupChannel2 != nil || stream.BackupChannel3 != nil {
					ThirdParty(streamID, playlistID, true, backupNumber)
				}
				return
			}

			var stream = playlist.Streams[streamID]

			if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {

				var clients = c.(structs.ClientConnection)
				clients.Error = err
				config.BufferClients.Store(playlistID+stream.MD5, clients)

			}

		}

		if err := config.BufferVFS.RemoveAll(storage.GetPlatformPath(tmpFolder)); err != nil {
			cli.ShowError(err, 4005)
		}

		err := storage.CheckVFSFolder(tmpFolder, config.BufferVFS)
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		err = storage.CheckFile(path)
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		cli.ShowInfo(fmt.Sprintf("%s path:%s", bufferType, path))
		cli.ShowInfo("Streaming URL:" + url)

		var tmpFile = fmt.Sprintf("%s%d.ts", tmpFolder, tmpSegment)

		f, err := config.BufferVFS.Create(tmpFile)
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		defer func() {
			err = f.Close()
		}()
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		// Set User-Agent
		var args []string

		for i, a := range strings.Split(options, " ") {

			switch bufferType {
			case "FFMPEG":
				a = strings.ReplaceAll(a, "[URL]", url)
				if i == 0 {
					if len(config.Settings.UserAgent) != 0 {
						args = []string{"-user_agent", config.Settings.UserAgent}
					}

					if playlist.HttpProxyIP != "" && playlist.HttpProxyPort != "" {
						args = append(args, "-http_proxy", fmt.Sprintf("http://%s:%s", playlist.HttpProxyIP, playlist.HttpProxyPort))
					}

					var headers string
					if len(playlist.HttpUserReferer) != 0 {
						headers += fmt.Sprintf("Referer: %s\r\n", playlist.HttpUserReferer)
					}
					if len(playlist.HttpUserOrigin) != 0 {
						headers += fmt.Sprintf("Origin: %s\r\n", playlist.HttpUserOrigin)
					}
					if headers != "" {
						args = append(args, "-headers", headers)
					}
				}

				args = append(args, a)

			case "VLC":
				if a == "[URL]" {
					a = strings.ReplaceAll(a, "[URL]", url)
					args = append(args, a)

					if len(config.Settings.UserAgent) != 0 {
						args = append(args, fmt.Sprintf(":http-user-agent=%s", config.Settings.UserAgent))
					}

					if len(playlist.HttpUserReferer) != 0 {
						args = append(args, fmt.Sprintf(":http-referrer=%s", playlist.HttpUserReferer))
					}

					if playlist.HttpProxyIP != "" && playlist.HttpProxyPort != "" {
						args = append(args, fmt.Sprintf(":http-proxy=%s:%s", playlist.HttpProxyIP, playlist.HttpProxyPort))
					}

				} else {
					args = append(args, a)
				}

			}

		}

		var cmd = exec.Command(path, args...)
		// Set this explicitly to avoid issues with VLC
		cmd.Env = append(os.Environ(), "DISPLAY=:0")

		debug = fmt.Sprintf("BUFFER DEBUG: %s:%s %s", bufferType, path, args)
		cli.ShowDebug(debug, 1)

		// Byte data from the process
		stdOut, err := cmd.StdoutPipe()
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		// Log data from the process
		logOut, err := cmd.StderrPipe()
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		if len(buf.Bytes()) == 0 && !stream.Status {
			cli.ShowInfo(bufferType + ":Processing data")
		}

		err = cmd.Start()
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		defer func() {
			err = cmd.Wait()
		}()
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		go func() {

			// Display log data from the process in debug mode 1.
			scanner := bufio.NewScanner(logOut)
			scanner.Split(bufio.ScanLines)

			for scanner.Scan() {

				debug = fmt.Sprintf("%s log:%s", bufferType, strings.TrimSpace(scanner.Text()))

				select {
				case <-streamStatus:
					cli.ShowDebug(debug, 1)
				default:
					cli.ShowInfo(debug)
				}

				time.Sleep(time.Duration(10) * time.Millisecond)

			}

		}()

		f, err = config.BufferVFS.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		defer func() {
			err = f.Close()
		}()
		if err != nil {
			cli.ShowError(err, 0)
			client.KillConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		buffer := make([]byte, 1024*4)

		reader := bufio.NewReader(stdOut)

		t := make(chan int)

		go func() {

			var timeout = 0
			for {
				time.Sleep(time.Duration(1000) * time.Millisecond)
				timeout++

				select {
				case <-t:
					return
				default:
					// Check if the channel is closed before sending
					select {
					case t <- timeout:
					default:
					}
				}

			}

		}()

		for {

			select {
			case timeout := <-t:
				if timeout >= 20 && tmpSegment == 1 {
					err = cmd.Process.Kill()
					if err != nil {
						cli.ShowError(err, 0)
					}
					err = errors.New("Timeout")
					cli.ShowError(err, 4006)
					client.KillConnection(streamID, playlistID, false)
					addErrorToStream(err)
					err = cmd.Wait()
					if err != nil {
						cli.ShowError(err, 0)
					}
					err = f.Close()
					if err != nil {
						cli.ShowError(err, 0)
					}
					return
				}

			default:

			}

			if fileSize == 0 && !stream.Status {
				cli.ShowInfo("Streaming Status:Receive data from " + bufferType)
			}

			if !client.Connection(stream) {
				err = cmd.Process.Kill()
				if err != nil {
					cli.ShowError(err, 0)
				}

				err = f.Close()
				if err != nil {
					cli.ShowError(err, 0)
				}

				err = cmd.Wait()
				if err != nil {
					cli.ShowError(err, 0)
				}
				return
			}

			n, err := reader.Read(buffer)
			if err == io.EOF {
				break
			}

			fileSize = fileSize + len(buffer[:n])

			if _, err := f.Write(buffer[:n]); err != nil {
				cli.ShowError(err, 0)
				err = cmd.Process.Kill()
				if err != nil {
					cli.ShowError(err, 0)
				}
				client.KillConnection(streamID, playlistID, false)
				addErrorToStream(err)
				err = cmd.Wait()
				if err != nil {
					cli.ShowError(err, 0)
				}

				return
			}

			if fileSize >= bufferSize/2 {

				if tmpSegment == 1 && !stream.Status {
					close(t)
					close(streamStatus)
					cli.ShowInfo(fmt.Sprintf("Streaming Status:Buffering data from %s", bufferType))
				}

				err = f.Close()
				if err != nil {
					cli.ShowError(err, 0)
					return
				}

				tmpSegment++

				if !stream.Status {
					config.Lock.Lock()
					stream.Status = true
					playlist.Streams[streamID] = stream
					config.BufferInformation.Store(playlistID, playlist)
					config.Lock.Unlock()
				}

				tmpFile = fmt.Sprintf("%s%d.ts", tmpFolder, tmpSegment)

				fileSize = 0

				var errCreate, errOpen error
				_, errCreate = config.BufferVFS.Create(tmpFile)
				f, errOpen = config.BufferVFS.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0600)
				if errCreate != nil || errOpen != nil {
					cli.ShowError(err, 0)
					err = cmd.Process.Kill()
					if err != nil {
						cli.ShowError(err, 0)
					}

					client.KillConnection(streamID, playlistID, false)
					addErrorToStream(err)
					err = cmd.Wait()
					if err != nil {
						cli.ShowError(err, 0)
					}
					return
				}

			}

		}

		err = cmd.Process.Kill()
		if err != nil {
			cli.ShowError(err, 0)
		}

		err = cmd.Wait()
		if err != nil {
			cli.ShowError(err, 0)
		}

		err = errors.New(bufferType + " error")
		addErrorToStream(err)
		cli.ShowError(err, 1204)

		time.Sleep(time.Duration(500) * time.Millisecond)
		client.Connection(stream)

		return

	}

}
