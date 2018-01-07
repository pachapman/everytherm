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
	"time"

	"github.com/kidoman/embd"
)

// A function that configures and then continually checks the reading of a thermocouple sensor via SPI.  Call this as a
// goroutine as it holds an infinite loop.
func MonitorTemperature(device *ETDevice) {
	// Initialize SPI
	if err := embd.InitSPI(); err != nil {
		device.Err = err
	}
	defer embd.CloseSPI()

	// Open a new bus through which we can read data
	spiBus := embd.NewSPIBus(embd.SPIMode0, 0, 1000000, 8, 0)
	defer spiBus.Close()

	for {
		dataReceived, err := spiBus.ReceiveData(3)
		device.Lock()
		if err != nil {
			device.Err = err
		} else {
			device.TempReading = dataReceived[0]
			if device.BluetoothMAC != "" {
				// We can only report the temperature once we've determined our BlueTooth MAC since we use that as a
				// unique identifier.  It's OK to do this asynchronously.  If the call eventually fails, we simply
				// wait until the next run to report it.
				go ReportReading(device)
			}
		}
		device.Unlock()
		time.Sleep(time.Second * 30)
	}
}
