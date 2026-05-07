package redactor

import (
	"net/url"
	"regexp"
	"strings"
)

type Redactor struct {
	patterns []*regexp.Regexp
}

func New() Redactor {
	keys := `(?i)(token|secret|api[_-]?key|apikey|authorization|bearer|password|passwd|pwd|phone|mobile|key)`
	return Redactor{patterns: []*regexp.Regexp{
		regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_-]{8,})`),
		regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._~+/=-]{8,}`),
		regexp.MustCompile(`(?i)(` + keys + `)(["'\s:=]+)([^"'\s,&}]+)`),
		regexp.MustCompile(`(?i)([?&](?:api_?key|key|token|secret|access_token|auth)=)([^&\s]+)`),
		regexp.MustCompile(`(?i)([A-Za-z0-9_-]*(?:token|secret|apikey|api_key|password)[A-Za-z0-9_-]*=)([^&\s]+)`),
		regexp.MustCompile(`\b1[3-9]\d{9}\b`),
	}}
}

func (r Redactor) Redact(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = redactURLs(out)
	for _, p := range r.patterns {
		out = p.ReplaceAllStringFunc(out, func(match string) string {
			if strings.HasPrefix(match, "1") && len(match) == 11 {
				return "1**********"
			}
			if strings.Contains(strings.ToLower(match), "bearer ") {
				return regexp.MustCompile(`(?i)(Bearer\s+).*`).ReplaceAllString(match, "${1}*****")
			}
			if strings.HasPrefix(match, "sk-") || strings.HasPrefix(strings.ToLower(match), "sk-") {
				return "sk-*****"
			}
			if strings.Contains(match, "?") || strings.Contains(match, "&") {
				return regexp.MustCompile(`=([^&\s]+)`).ReplaceAllString(match, "=*****")
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

func redactURLs(s string) string {
	re := regexp.MustCompile(`https?://[^\s"'<>]+`)
	return re.ReplaceAllStringFunc(s, func(raw string) string {
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
