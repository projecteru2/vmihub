package utils

import "strings"

const (
	FakeTag = "0000000000"
)

func IsDefaultTag(tag string) bool {
	return tag == "" || tag == "latest"
}

func NormalizeTag(tag string, digest string) string {
	if IsDefaultTag(tag) {
		if digest == "" {
			return FakeTag
		}
		return strings.TrimPrefix(digest, "sha256:")[0:10]
	}
	return tag
}
