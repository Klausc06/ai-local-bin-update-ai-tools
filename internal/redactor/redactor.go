package redactor

import (
	"net/url"
	"regexp"
	"strings"
)

type Redactor struct {
	patterns     []*regexp.Regexp
	urlExtractRe *regexp.Regexp
	bearerRe     *regexp.Regexp
	urlParamRe   *regexp.Regexp
	ansiRe       *regexp.Regexp
}

func New() Redactor {
	keys := `(?i)(token|secret|api[_-]?key|apikey|authorization|bearer|password|passwd|pwd|phone|mobile|key)`
	return Redactor{
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_-]{8,})`),
			regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._~+/=-]{8,}`),
			regexp.MustCompile(`(?i)(` + keys + `)(["'\s:=]+)([^"'\s,&}]+)`),
			regexp.MustCompile(`(?i)([?&](?:api_?key|key|token|secret|access_token|auth)=)([^&\s]+)`),
			regexp.MustCompile(`(?i)([A-Za-z0-9_-]*(?:token|secret|apikey|api_key|password)[A-Za-z0-9_-]*=)([^&\s]+)`),
			regexp.MustCompile(`\b1[3-9]\d{9}\b`),
		},
		urlExtractRe: regexp.MustCompile(`https?://[^\s"'<>]+`),
		bearerRe:     regexp.MustCompile(`(?i)(Bearer\s+).*`),
		urlParamRe:   regexp.MustCompile(`=([^&\s]+)`),
		ansiRe:       regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`),
	}
}

func (r Redactor) Redact(s string) string {
	if s == "" {
		return s
	}
	out := r.ansiRe.ReplaceAllString(s, "")
	out = r.redactURLs(out)
	for _, p := range r.patterns {
		out = p.ReplaceAllStringFunc(out, func(match string) string {
			if strings.HasPrefix(match, "1") && len(match) == 11 {
				return "1**********"
			}
			if strings.Contains(strings.ToLower(match), "bearer ") {
				return r.bearerRe.ReplaceAllString(match, "${1}*****")
			}
			if strings.HasPrefix(match, "sk-") || strings.HasPrefix(strings.ToLower(match), "sk-") {
				return "sk-*****"
			}
			if strings.Contains(match, "?") || strings.Contains(match, "&") {
				return r.urlParamRe.ReplaceAllString(match, "=*****")
			}
			idx := strings.LastIndexAny(match, "=: ")
			if idx >= 0 {
				return match[:idx+1] + "*****"
			}
			return "*****"
		})
	}
	return out
}

func (r Redactor) redactURLs(s string) string {
	return r.urlExtractRe.ReplaceAllStringFunc(s, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		q := u.Query()
		changed := false
		for key := range q {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "auth") {
				q.Set(key, "*****")
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
		}
		return u.String()
	})
}
