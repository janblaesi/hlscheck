hlscheck
===

**hlscheck** is a small utility that may be used for monitoring a HLS stream over a long period of time.
It will not check the audio itself but only make sure that all segments that are referenced are present and
served by the server at the time they are added. A log entry will be written when fetching fails.

The utility is aware of multi-rendition playlists and will monitor all renditions simultaneously.

Usage
---

**hlscheck** is a command line program that only needs to be fed an URL and an optional path for a log file to run:

```
hlscheck -url <stream url> [-logfile <logfile path>]
```
