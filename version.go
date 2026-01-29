package e2b

import (
	"golang.org/x/mod/semver"
)

// compareVersions compares two semantic version strings, handling versions
// with or without the "v" prefix (e.g., both "1.0.0" and "v1.0.0" are valid).
// Returns -1 if v1 < v2, 0 if equal, 1 if v1 > v2.
// Returns -1 if v1 is empty.
func compareVersions(v1, v2 string) int {
	if v1 == "" {
		return -1
	}
	if v1[0] != 'v' {
		v1 = "v" + v1
	}
	if v2 != "" && v2[0] != 'v' {
		v2 = "v" + v2
	}
	return semver.Compare(v1, v2)
}
