package storage

import "strings"

func filterVersionNumber(v string) string {
	return strings.TrimPrefix(v, "v")
}
