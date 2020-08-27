package utils

import "regexp"

var (
	// semanticVersionMatcher is taken from https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
	SemanticVersionMatcher = regexp.MustCompile(`v(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?`)
)

func RegexCapture(r *regexp.Regexp, s string) (groups map[string]string) {
	match := r.FindStringSubmatch(s)

	groups = make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i == 0 && len(match) > 0 {
			groups["full_match"] = match[i]
		}
		if i > 0 && i <= len(match) {
			groups[name] = match[i]
		}
	}

	return
}
