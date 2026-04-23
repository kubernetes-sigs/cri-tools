/*
Originally from https://github.com/moby/moby/blob/v2.0.0-beta.9/client/internal/timestamp/timestamp.go
Copyright (C) Docker/Moby authors.
Licensed under the Apache License, Version 2.0
NOTICE: https://github.com/moby/moby/blob/v2.0.0-beta.9/NOTICE
*/

package timetype

import (
	"strconv"
	"testing"
	"time"
)

func TestGetTimestampDuration(t *testing.T) {
	t.Parallel()

	reference := time.Unix(1700000000, 0).UTC()

	got, err := GetTimestamp("42m", reference)
	if err != nil {
		t.Fatalf("GetTimestamp returned error: %v", err)
	}

	want := strconv.FormatInt(reference.Add(-42*time.Minute).Unix(), 10)
	if got != want {
		t.Fatalf("GetTimestamp() = %q, want %q", got, want)
	}
}

func TestGetTimestampRFC3339WithoutZoneUsesReferenceLocation(t *testing.T) {
	t.Parallel()

	reference := time.Date(2026, 4, 21, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	got, err := GetTimestamp("2013-01-02T13:23:37", reference)
	if err != nil {
		t.Fatalf("GetTimestamp returned error: %v", err)
	}

	want := "1357104217.000000000"
	if got != want {
		t.Fatalf("GetTimestamp() = %q, want %q", got, want)
	}
}

func TestGetTimestampUnixValuePassThrough(t *testing.T) {
	t.Parallel()

	reference := time.Unix(1700000000, 0).UTC()

	got, err := GetTimestamp("1136073600.000000001", reference)
	if err != nil {
		t.Fatalf("GetTimestamp returned error: %v", err)
	}

	if got != "1136073600.000000001" {
		t.Fatalf("GetTimestamp() = %q, want unix timestamp pass-through", got)
	}
}

func TestGetTimestampInvalid(t *testing.T) {
	t.Parallel()

	reference := time.Unix(1700000000, 0).UTC()

	if _, err := GetTimestamp("invalid", reference); err == nil {
		t.Fatal("GetTimestamp(invalid) = nil error, want error")
	}
}

func TestParseTimestamps(t *testing.T) {
	t.Parallel()

	seconds, nanos, err := ParseTimestamps("1136073600.000000001", 0)
	if err != nil {
		t.Fatalf("ParseTimestamps returned error: %v", err)
	}

	if seconds != 1136073600 || nanos != 1 {
		t.Fatalf("ParseTimestamps() = (%d, %d), want (1136073600, 1)", seconds, nanos)
	}
}

func TestParseTimestampsEmpty(t *testing.T) {
	t.Parallel()

	seconds, nanos, err := ParseTimestamps("", 123)
	if err != nil {
		t.Fatalf("ParseTimestamps returned error: %v", err)
	}

	if seconds != 123 || nanos != 0 {
		t.Fatalf("ParseTimestamps() = (%d, %d), want (123, 0)", seconds, nanos)
	}
}
