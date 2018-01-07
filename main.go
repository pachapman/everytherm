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

// Entry point to EveryTherm Embedded Application.
package main

import (
	"flag"
	"time"

	"github.com/skynetservices/skynet/log"

	"github.com/pchapman/everytherm/etlib"
)

// Initialization parameters to come from command line along with reasonable defaults.
var (
	mc    = flag.Int("mc", 1, "Maximum concurrent connections")
	id    = flag.Duration("id", 0, "ibeacon duration")
	ii    = flag.Duration("ii", 5*time.Second, "ibeacon interval")
	name  = flag.String("name", "EveryTherm Sensor", "Device Name")
	dev   = flag.Int("dev", -1, "HCI device ID")
	chk   = flag.Bool("chk", true, "Check device LE support")

	loggingHost = flag.String("loggingHost", "127.0.0.1", "The host to connect to for syslog logging.")
	loggingLvl = flag.String("loggingLevel", "INFO", "The level of logging to write to syslog.")
	loggingPort = flag.Int("loggingPort", 514, "The port to connect to for syslog logging.")
)

func main() {
	flag.Parse()

	// Configure Logging
	log.SetSyslogHost(*loggingHost)
	log.SetSyslogPort(*loggingPort)
	log.SetLogLevel(log.LevelFromString(*loggingLvl))
	log.Initialize()
	log.Println(log.INFO, "Configuring the EveryTherm Service.")

	device := etlib.ETDevice{BluetoothMAC:"", TempReading:0, Err:nil}
	go etlib.MonitorTemperature(&device)
	err := etlib.InitializeBluetoothServices(*mc, *id, *ii, *name, *dev, *chk, &device)
	if err != nil {
		sys.Exit(1)
	}

	select {}
}
