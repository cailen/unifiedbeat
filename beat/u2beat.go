/* Copyright (c) 2016 Chris Smith
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED ``AS IS'' AND ANY EXPRESS OR IMPLIED
 * WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
 * IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

package unifiedbeat

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/cfgfile"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

// The "quit" channel is used to tell
// U2SpoolAndPublish to stop gracefully. This
// ensures the registry file is up-to-date.
// var quit chan bool

type Unifiedbeat struct {
	UbConfig     ConfigSettings
	registrar    *Registrar
	isSpooling   bool
	spoolTimeout time.Duration
}

func New(b *beat.Beat, _ *common.Config) (beat.Beater, error) {
	return &Unifiedbeat{}, nil
}

func (ub *Unifiedbeat) Config(b *beat.Beat) error {
	// load the unifiedbeat.yml config file
	err := cfgfile.Read(&ub.UbConfig, "")
	if err != nil {
		return fmt.Errorf("Error reading config file: %v", err)
	}
	return nil
}

func (ub *Unifiedbeat) Setup(b *beat.Beat) error {
	// Go overboard checking stuff . . .

	// It is possible for the Unified2Path to contain no files, as
	// there may have been no sensor alerts/events yet, so we
	// can not verify Unified2Prefix only that Unified2Path is valid.

	u2PathPrefixSettings := path.Join(ub.UbConfig.Sensor.Unified2Path, ub.UbConfig.Sensor.Unified2Prefix)
	// disallow filename globbing (remove all trailing *'s):
	u2PathPrefixSettings = strings.TrimRight(u2PathPrefixSettings, "*")
	// make path absolute (as it may be relative in unifiedbeat.yml):
	absPath, err := filepath.Abs(u2PathPrefixSettings)
	if err != nil {
		// this is not really an error, but it should NOT happen:
		logp.Info("Setup: failed to set the absolute path for unified2 files: '%s'", u2PathPrefixSettings)
		absPath = u2PathPrefixSettings // whatever, just use it as-is
	}
	// ensure folder exists:
	ub.UbConfig.Sensor.Spooler.Folder = path.Dir(absPath)
	_, err = os.Stat(ub.UbConfig.Sensor.Spooler.Folder)
	if err != nil {
		// unable to find the unified2 files folder:
		logp.Critical("Setup: ERROR: 'unified2_path' is an invalid path; correct the YAML config file!")
		os.Exit(1)
	}
	ub.UbConfig.Sensor.Spooler.FilePrefix = path.Base(absPath)

	if len(ub.UbConfig.Sensor.Rules.GenMsgMapPath) == 0 {
		logp.Critical("Setup: ERROR: required path to 'gen_msg_map_path' not specified in YAML config file!")
		os.Exit(1)
	}
	if len(ub.UbConfig.Sensor.Rules.Paths) == 0 {
		logp.Critical("Setup: ERROR: required path(s) to Rule files not specified in YAML config file!")
		os.Exit(1)
	}

	if ub.UbConfig.Sensor.Geoip2Path == "" {
		logp.Info("Setup: 'geoip2_path:' not specified in YAML config file.")
	} else {
		// prefer to use GeoIP2 databases for geocoding both IPv4/6 addresses:
		err := OpenGeoIp2DB(ub.UbConfig.Sensor.Geoip2Path)
		if err != nil {
			logp.Critical("Setup: failed opening 'GeoIp2' database; error: %v", err)
			os.Exit(1)
		}
		logp.Info("Setup: activated 'GeoIP2' database for IP v4 and v6 geolocating.")
	}

	// load Rules and SourceFiles:
	multipleLineWarnings, duplicateRuleWarnings, err := LoadRules(ub.UbConfig.Sensor.Rules.GenMsgMapPath, ub.UbConfig.Sensor.Rules.Paths)
	if err != nil {
		logp.Critical("Setup: loading Rules error: %v", err)
		os.Exit(1)
	}
	logp.Info("Setup: Rules warnings: %v multiple line rules rejected, %v duplicate rules rejected", multipleLineWarnings, duplicateRuleWarnings)
	logp.Info("Setup: Rules stats: %v rule files read, %v rules created", len(SourceFiles), len(Rules))

	ub.spoolTimeout = time.Duration(5) * time.Second // default is 5 seconds
	if ub.UbConfig.Sensor.SpoolerTimeout > 0 {
		ub.spoolTimeout = time.Duration(ub.UbConfig.Sensor.SpoolerTimeout) * time.Second
	}

	// registry file is created in the current working directory:
	ub.registrar, err = NewRegistrar(".unifiedbeat")
	if err != nil {
		logp.Critical("Setup: unable to set registry file error: %v", err)
		os.Exit(1)
	}
	ub.registrar.LoadState()
	logp.Info("Setup: registrar: registry file: %#v", ub.registrar.registryFile)
	logp.Info("Setup: registrar: file source: %#v", ub.registrar.State.Source)
	logp.Info("Setup: registrar: file offset: %#v", ub.registrar.State.Offset)

	return nil
}

func (ub *Unifiedbeat) Run(b *beat.Beat) error {
	logp.Info("Run: start spooling and publishing...")

	// thoughts:
	//
	// 1. the "quit" channel is complicating things,
	//    and it seems wrongheaded to use a channel
	//    when there are no go routines, well actually
	//    libbeat start this beat as a go routine
	//    - so look at how other beats handle
	//      global-like vars (IsRunning)
	//
	// 2. what about using a IsRunning bool in the
	//    Unifiedbeat struct "ub.IsRunning", which
	//    ub.U2SpoolAndPublish() can access/change
	//    - only one U2SpoolAndPublish is ever running,
	//      so there's no race/mutex issues

	// use a channel to gracefully shutdown "U2SpoolAndPublish":
	// quit = make(chan bool)

	ub.isSpooling = true
	client, err := b.Publisher.Connect()
	if err != nil {
		return err
	}

	ub.U2SpoolAndPublish(client, b)

	// indicate that "U2SpoolAndPublish" returned unexpectedly,
	// and that it is no longer running, so the "quit" code is ignored:
	ub.isSpooling = false

	// do a WriteRegistry as the "quit" channel code may fail,
	// block, or whatever ... the worst is two writes of the
	// same info to the registry file:
	err2 := ub.registrar.WriteRegistry()
	if err != nil {
		logp.Info("Run: failed to update registry file; error: %v", err)
		return err2 // return to "main.go" after Stop() and Cleanup()
	}
	logp.Info("Run: updated registry file.")

	// returning always calls Stop and Cleanup, and in that order
	return nil // return to "main.go" after Stop() and Cleanup()
}

// Stop is called on exit before Cleanup
// why isn't the flow Cleanup and then Stop?
func (ub *Unifiedbeat) Stop() {
	startStopping := time.Now()
	logp.Info("Stop: is spooling and publishing running? '%v'", ub.isSpooling)
	if ub.isSpooling {
		ub.isSpooling = false
		logp.Info("Stop: waiting %v for spool/publish to shutdown.", ub.spoolTimeout)

		// lame, but might work
		time.Sleep(ub.spoolTimeout)

		// // tell "U2SpoolAndPublish" to shutdown:
		// quit <- true
		// // block/wait for "U2SpoolAndPublish" to close the quit channel:
		// <-quit

		err := ub.registrar.WriteRegistry()
		if err != nil {
			logp.Info("Stop: failed to update registry file; error: %v", err)
		} else {
			logp.Info("Stop: successfully updated registry file.")
		}
	}
	elapsed := time.Since(startStopping)
	logp.Info("Stop: done after waiting %v.", elapsed)
}

func (ub *Unifiedbeat) Cleanup(b *beat.Beat) error {
	logp.Info("Cleanup: is spooling and publishing running? '%v'", ub.isSpooling)
	// see "beat/geoip2.go":
	if GeoIp2Reader != nil {
		GeoIp2Reader.Close()
		logp.Info("Cleanup: closed GeoIp2Reader.")
	}
	logp.Info("Cleanup: done.")
	return nil
}
