package host

import (
	"net"
	"os"
	"strings"
	"threadfin/internal/config"
)

func ResolveIP() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			networkIP, ok := addr.(*net.IPNet)
			config.System.IPAddressesList = append(config.System.IPAddressesList, networkIP.IP.String())

			if ok {
				ip := networkIP.IP.String()

				if networkIP.IP.To4() != nil {
					// Skip unwanted IPs
					if !strings.HasPrefix(ip, "169.254") {
						config.System.IPAddressesV4 = append(config.System.IPAddressesV4, ip)
						config.System.IPAddress = ip
					}
				} else {
					config.System.IPAddressesV6 = append(config.System.IPAddressesV6, ip)
				}
			}
		}
	}

	if len(config.System.IPAddress) == 0 {
		if len(config.System.IPAddressesV4) > 0 {
			config.System.IPAddress = config.System.IPAddressesV4[0]
		} else if len(config.System.IPAddressesV6) > 0 {
			config.System.IPAddress = config.System.IPAddressesV6[0]
		}
	}

	config.System.Hostname, err = os.Hostname()
	if err != nil {
		return err
	}

	return nil
}
