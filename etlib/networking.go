/*
 * Copyright © 2017 PCSW Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package etlib

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strings"

	"github.com/skynetservices/skynet/log"
)

const networkStatusDown = "DOWN"
const networkStatusLocal = "LOCAL"
const networkStatusUP = "UP"
const serviceURL = "http://services.pcsw.us/everytherm/report?device=%s&reading=&d"
const ssidOff = "off/any"
const wpaConfigFile = "/etc/wpa_supplicant/wpa_supplicant.conf"
const wpaConfigText =
    "country=US\n" +
    "ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev\n" +
    "update_config=1\n" +
    "network={\n" +
	"	ssid=\"%s\"\n" +
    "   psk=\"%s\"\n" +
    "}"

// A struct which holds information to be used to configure the WiFi network interface
type NetworkConfig struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

// A struct which holds information about the WiFi network's current state
type NetworkInfo struct {
	SSID        string `json:"ssid"`
	IPAddr      string `json:"ipaddr"`
	Status      string `json:"status"`
	Available []string `json:"available"`
}

// Configures the WiFi network interface given the provided configuration
func ConfigureNetwork(config NetworkConfig) (error) {
	s := fmt.Sprintf(wpaConfigText, config.SSID, config.Password)
	err := ioutil.WriteFile(wpaConfigFile, []byte(s), 0600)
	if err != nil {
		return err
	} else {
		go reconfigureNetwork()
		return nil
	}
}

// Issues the cli command to restart the WiFi interface with the new configuration
func reconfigureNetwork() {
	cmd := exec.Command("wpa_cli", "-i", "wlan0", "reconfigure")
	cmd.Run()
}

// Gets the current state of the WiFi network.
func GetNetworkInfo() (NetworkInfo) {
	info := NetworkInfo{"", "",networkStatusDown, []string{}}
	s, err := getConnectedSSID()
	if err != nil {
		log.Errorf("%s", err)
		info.SSID = "UNKNOWN"
	} else {
		if s == ssidOff {
			info.SSID = ""
			info.Status = networkStatusDown
		} else {
			info.SSID = s
			s, err = getIPAddress()
			if err != nil {
				log.Errorf("error obtaining IP address: %s", err)
				info.IPAddr = ""
			} else {
				info.IPAddr = s
				if isServerReachable() {
					info.Status = networkStatusUP
				} else {
					info.Status = networkStatusLocal
				}
			}
		}
	}
	var ssids []string
	ssids, err = getAvailableSSIDs()
	if err != nil {
		log.Errorf("error determining available SSIDs: %s", err)
		info.Available = []string{}
	} else {
		info.Available = ssids
	}
	return info
}

// Makes cli call to iwlist to get a list of available WiFi SSIDs
func getAvailableSSIDs() ([]string, error) {
	ssids := make([]string, 0, 0)
	iwlistCmd := exec.Command("iwlist", "wlan0", "scan")
	iwlistCmdOut, err := iwlistCmd.Output()
	if err != nil {
		return []string{}, fmt.Errorf("error when getting the interface information: %s", err)
	} else {
		lines := strings.Split(string(iwlistCmdOut), "\n")
		for _, line := range lines {
			i := strings.Index(line, "ESSID")
			if i > -1 {
				parts := strings.Split(line, "\"")
				log.Infof("Found SSID %s", parts[1])
				ssids = append(ssids, parts[1])
			}
		}
	}
	return ssids, nil
}

// Uses the cli command iwconfig to get the name of the connected SSID, if any
func getConnectedSSID() (string, error) {
	iwcfgCmd := exec.Command("iwconfig", "wlan0")
	iwcfgCmdOut, err := iwcfgCmd.Output()
	if err != nil {
		return "", fmt.Errorf("error when getting the interface information: %s", err)
	} else {
		lines := strings.Split(string(iwcfgCmdOut), "\n")
		for _, line := range lines {
			i := strings.Index(line, "ESSID")
			if i > -1 {
				parts := strings.Split(line, ":")
				log.Infof("Connected to SSID %s", parts[1])
				return parts[1], nil
			}
		}
	}
	return "", nil
}

// Gets the IP address of the default outgoing interface.  We assume it's the WiFi interface but it could be the
// ehternet interface if they plug it into the network.
func getIPAddress() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String(), nil
}

// Determines if we can reach hosts outside the network.
func isServerReachable() (bool) {
	resp, err := http.Get("https://google.com")
	if err != nil && resp.StatusCode == http.StatusOK {
		return true
	} else {
		return false
	}
}

// Makes a call to the web service to report the latest reading.
func ReportReading(device *ETDevice) (error) {
	resp, err := http.Post(fmt.Sprintf(serviceURL, device.BluetoothMAC, device.TempReading))
	if err != nil && resp.StatusCode == http.StatusOK {
		return true
	} else {
		return false
	}
}
