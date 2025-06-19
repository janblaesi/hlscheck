/*
	Copyright 2025 Jan Blaesi

	Permission is hereby granted, free of charge, to any person obtaining a copy of this software
	and associated documentation files (the “Software”), to deal in the Software without
	restriction, including without limitation the rights to use, copy, modify, merge, publish,
	distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the
	Software is furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all copies or
	substantial portions of the Software.

	THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
	THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
	OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
	ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
	OTHER DEALINGS IN THE SOFTWARE.
*/

package main

import (
	"flag"
	"hlscheck/checker"
	"hlscheck/plist"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var streamUrl string
	var logPath string

	flag.StringVar(&streamUrl, "url", "", "URL of the stream to check (to the master playlist)")
	flag.StringVar(&logPath, "logfile", "", "Log file to redirect the output of the program to")
	flag.Parse()

	if streamUrl == "" {
		slog.Error("Missing stream URL!")
		os.Exit(1)
	}
	if logPath != "" {
		SetupLogfile(logPath)
	}

	StartPlaylistChecker(streamUrl)

	// Wait for a SIGINT or SIGTERM signal to stop the application.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}

// SetupLogfile will add a new slog handler to write the log output to file.
func SetupLogfile(logPath string) {
	logFileHandle, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Unable to open log file", "err", err)
		os.Exit(1)
	}

	logHandle := slog.New(slog.NewTextHandler(logFileHandle, nil))
	slog.SetDefault(logHandle)
}

// StartPlaylistChecker will fetch the user-provided URL and start a checker instance for each variant playlist.
func StartPlaylistChecker(url string) {
	pl := plist.Plist{}
	if err := plist.FetchAndParse(&pl, url); err != nil {
		slog.Error("Fetching playlist failed", "err", err)
		os.Exit(1)
	}

	switch pl.Type {
	case plist.MasterPlist:
		for _, plEntry := range pl.Entries {
			checker.New(plEntry.URL)
		}
	case plist.VariantPlist:
		checker.New(url)
	}
}
