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

// Bluetooth module for the EveryTherm sensor device.
package etlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
	"os"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/linux/cmd"
	"github.com/skynetservices/skynet/log"
)

var BluetoothServiceUUID = gatt.MustParseUUID("d9f99901-97cd-4506-ae8b-ceecf44b84c4")
var ConfigServiceUUID = gatt.MustParseUUID("d9f99901-97cd-4506-ae8b-ceecf44b84c5")
var SensorServiceUUID = gatt.MustParseUUID("d9f99901-97cd-4506-ae8b-ceecf44b84c6")
var ServiceUUIDs = []gatt.UUID{BluetoothServiceUUID}

// Sets up a new service that followed the EveryTherm protocol.
func NewETLIBService(device *ETDevice) *gatt.Service {

	// Configure Characteristic
	// Output to Bluetooth client
	s := gatt.NewService(BluetoothServiceUUID)
	s.AddCharacteristic(ConfigServiceUUID).HandleReadFunc(
		func(rsp gatt.ResponseWriter, req *gatt.ReadRequest) {
			info := GetNetworkInfo()
			var data, err = json.Marshal(info)
			if err != nil {
				log.Errorf("Error marshalling network info for reporting: %s", err)
				rsp.SetStatus(gatt.StatusUnexpectedError)
			} else {
				fmt.Fprint(rsp, data)
				rsp.SetStatus(gatt.StatusSuccess)
			}
		})
	// Input from Bluetooth client
	s.AddCharacteristic(ConfigServiceUUID).HandleWriteFunc(
		func(r gatt.Request, data []byte) (status byte) {
			var config = NetworkConfig{}
			err := json.Unmarshal(data, config)
			if err != nil {
				log.Errorf("Error unmarshalling network config from provider: %s", err)
				return gatt.StatusUnexpectedError
			} else {
				err = ConfigureNetwork(config)
				if err != nil {
					log.Errorf("Error configuring network: %s", err)
					return gatt.StatusUnexpectedError
				} else {
					return gatt.StatusSuccess
				}
			}
		})

	// Sensor Characteristic
	s.AddCharacteristic(SensorServiceUUID).HandleNotifyFunc(
		func(r gatt.Request, n gatt.Notifier) {
			for !n.Done() {
				if device.Err == nil {
					fmt.Fprintf(n, "%d", device.TempReading)
				} else {
					fmt.Fprint(n, "ERROR")
				}
				time.Sleep(time.Second * 30)
			}
		})

	return s
}

// cmdReadBDAddr implements cmd.CmdParam for LnxSendHCIRawCommand()
type cmdReadBDAddr struct{}

func (c cmdReadBDAddr) Marshal(b []byte) {}
func (c cmdReadBDAddr) Opcode() int      { return 0x1009 }
func (c cmdReadBDAddr) Len() int         { return 0 }

// Get bluetooth address with LnxSendHCIRawCommand()
func getBluetoothAddress(d gatt.Device) (string, error) {
	rsp := bytes.NewBuffer(nil)
	if err := d.Option(gatt.LnxSendHCIRawCommand(&cmdReadBDAddr{}, rsp)); err != nil {
		return "", fmt.Errorf("failed to send HCI raw command, err: %s", err)
	}
	b := rsp.Bytes()
	if b[0] != 0 {
		return "", fmt.Errorf("failed to get bdaddr with HCI Raw command, status: %d", b[0])
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[6], b[5], b[4], b[3], b[2], b[1]), nil
}

// Sets up BlueTooth device and configures BlueTooth service and characteristics.
func InitializeBluetoothServices(mc int, id time.Duration, ii time.Duration, name string, dev int, chk bool, device *ETDevice) (error) {
	d, err := gatt.NewDevice(
		gatt.LnxMaxConnections(mc),
		gatt.LnxDeviceID(dev, chk),
		gatt.LnxSetAdvertisingParameters(&cmd.LESetAdvertisingParameters{
			AdvertisingIntervalMin: 0x00f4,
			AdvertisingIntervalMax: 0x00f4,
			AdvertisingChannelMap:  0x07,
		}),
	)

	if err != nil {
		log.Printf(log.ERROR, "Failed to open bluetooth device, err: %s", err)
		return err
	}

	// Register optional handlers.
	d.Handle(
		gatt.CentralConnected(func(c gatt.Central) { log.Infof("Connect: %s", c.ID()) }),
		gatt.CentralDisconnected(func(c gatt.Central) { log.Infof("Disconnect: %s", c.ID()) }),
	)

	// A mandatory handler for monitoring device state.
	onStateChanged := func(d gatt.Device, s gatt.State) {
		log.Printf(log.INFO,"State: %s\n", s)
		switch s {
		case gatt.StatePoweredOn:
			// Get bdaddr with LnxSendHCIRawCommand()
			bluetoothid, err := getBluetoothAddress(d)
			if err != nil {
				log.Printf(log.ERROR, "Unable to obtain bluetooth device ID, err: %s", err)
				os.Exit(1)
			} else {
				device.Lock()
				device.BluetoothMAC = bluetoothid
				device.Unlock()
			}

			// Setup GAP and GATT services.
			d.AddService(NewETLIBService(device))

			// If id is zero, advertise name and services statically.
			if id == time.Duration(0) {
				d.AdvertiseNameAndServices(name, ServiceUUIDs)
				break
			}

			// If id is non-zero, advertise name and services and iBeacon alternately.
			go func() {
				for {
					// Advertise as a RedBear Labs iBeacon.
					d.AdvertiseIBeacon(gatt.MustParseUUID("5AFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"), 1, 2, -59)
					time.Sleep(id)

					// Advertise name and services.
					d.AdvertiseNameAndServices(name, ServiceUUIDs)
					time.Sleep(ii)
				}
			}()

		default:
		}
	}

	d.Init(onStateChanged)

	return nil
}
