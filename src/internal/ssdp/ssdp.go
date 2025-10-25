package ssdp

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"time"

	"github.com/koron/go-ssdp"
)

// SSDP : SSPD / DLNA Server
func Start() (err error) {
	if !config.Settings.SSDP || config.System.Flag.Info {
		return
	}

	cli.ShowInfo(fmt.Sprintf("SSDP / DLNA:%t", config.Settings.SSDP))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	ad, err := ssdp.Advertise(
		"upnp:rootdevice", // send as "ST"
		fmt.Sprintf("uuid:%s::upnp:rootdevice", config.System.DeviceID), // send as "USN"
		fmt.Sprintf("%s/device.xml", config.System.URLBase),             // send as "LOCATION"
		config.System.AppName, // send as "SERVER"
		1800)                  // send as "maxAge" in "CACHE-CONTROL"

	if err != nil {
		return
	}

	// Debug SSDP
	if config.System.Flag.Debug == 3 {
		ssdp.Logger = log.New(os.Stderr, "[SSDP] ", log.LstdFlags)
	}

	go func(adv *ssdp.Advertiser) {

		aliveTick := time.NewTicker(300 * time.Second)

	loop:
		for {

			select {

			case <-aliveTick.C:
				err = adv.Alive()
				if err != nil {
					cli.ShowError(err, 0)
					err = adv.Bye()
					if err != nil {
						cli.ShowError(err, 0)
					}

					err = adv.Close()
					if err != nil {
						cli.ShowError(err, 0)
					}
					break loop
				}

			case <-quit:
				err = adv.Bye()
				if err != nil {
					cli.ShowError(err, 0)
				}
				err = adv.Close()
				if err != nil {
					cli.ShowError(err, 0)
				}
				os.Exit(0)
				break loop

			}

		}

	}(ad)

	return
}
