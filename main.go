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

	flag.StringVar(&streamUrl, "url", "", "URL of the stream to check (to the master playlist)")
	flag.Parse()

	if streamUrl == "" {
		slog.Error("Missing stream URL!")
		os.Exit(1)
	}

	StartPlaylistChecker(streamUrl)

	// Wait for a SIGINT or SIGTERM signal to stop the application.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
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
