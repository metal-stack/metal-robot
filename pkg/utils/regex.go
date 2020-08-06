package utils

import "regexp"

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
