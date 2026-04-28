/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Originally from https://github.com/moby/moby/blob/v2.0.0-beta.9/client/internal/timestamp/timestamp.go
Copyright (C) Docker/Moby authors.
Licensed under the Apache License, Version 2.0
NOTICE: https://github.com/moby/moby/blob/v2.0.0-beta.9/NOTICE
*/

package timetype

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	rfc3339Local     = "2006-01-02T15:04:05"
	rfc3339NanoLocal = "2006-01-02T15:04:05.999999999"
	dateWithZone     = "2006-01-02Z07:00"
	dateLocal        = "2006-01-02"
)

// GetTimestamp tries to parse given string as golang duration,
// then RFC3339 time and finally as a Unix timestamp. If
// any of these were successful, it returns a Unix timestamp
// as string otherwise returns the given value back.
// In case of duration input, the returned timestamp is computed
// as the given reference time minus the amount of the duration.
func GetTimestamp(value string, reference time.Time) (string, error) {
	if d, err := time.ParseDuration(value); value != "0" && err == nil {
		return strconv.FormatInt(reference.Add(-d).Unix(), 10), nil
	}

	var format string

	parseInLocation := !strings.ContainsAny(value, "zZ+") && strings.Count(value, "-") != 3

	switch {
	case strings.Contains(value, "."):
		if parseInLocation {
			format = rfc3339NanoLocal
		} else {
			format = time.RFC3339Nano
		}
	case strings.Contains(value, "T"):
		tcolons := strings.Count(value, ":")

		if !parseInLocation && !strings.ContainsAny(value, "zZ") && tcolons > 0 {
			tcolons--
		}

		if parseInLocation {
			switch tcolons {
			case 0:
				format = "2006-01-02T15"
			case 1:
				format = "2006-01-02T15:04"
			default:
				format = rfc3339Local
			}
		} else {
			switch tcolons {
			case 0:
				format = "2006-01-02T15Z07:00"
			case 1:
				format = "2006-01-02T15:04Z07:00"
			default:
				format = time.RFC3339
			}
		}
	case parseInLocation:
		format = dateLocal
	default:
		format = dateWithZone
	}

	var (
		parsed time.Time
		err    error
	)
	if parseInLocation {
		parsed, err = time.ParseInLocation(format, value, time.FixedZone(reference.Zone()))
	} else {
		parsed, err = time.Parse(format, value)
	}

	if err != nil {
		if strings.Contains(value, "-") {
			return "", err
		}

		if _, _, err := parseTimestamp(value); err != nil {
			return "", fmt.Errorf("failed to parse value as time or duration: %q", value)
		}

		return value, nil
	}

	return fmt.Sprintf("%d.%09d", parsed.Unix(), int64(parsed.Nanosecond())), nil
}

// ParseTimestamps returns seconds and nanoseconds from a timestamp in the
// format of "%d.%09d". It returns defaultSeconds if value is empty.
func ParseTimestamps(value string, defaultSeconds int64) (seconds, nanoseconds int64, _ error) {
	if value == "" {
		return defaultSeconds, 0, nil
	}

	return parseTimestamp(value)
}

func parseTimestamp(value string) (seconds, nanoseconds int64, _ error) {
	secondsText, nanosText, hasNanos := strings.Cut(value, ".")

	seconds, err := strconv.ParseInt(secondsText, 10, 64)
	if err != nil {
		return seconds, 0, err
	}

	if !hasNanos {
		return seconds, 0, nil
	}

	nanos, err := strconv.ParseInt(nanosText, 10, 64)
	if err != nil {
		return seconds, nanos, err
	}

	nanos = int64(float64(nanos) * math.Pow(10, float64(9-len(nanosText))))

	return seconds, nanos, nil
}
