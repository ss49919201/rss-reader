package rss

import (
	"html"
	"net/url"
	"strings"
	"unicode"
)

func stripTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		lt := strings.IndexByte(s[i:], '<')
		if lt < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+lt])
		i += lt
		if strings.HasPrefix(s[i:], "<!--") {
			if end := strings.Index(s[i:], "-->"); end >= 0 {
				i += end + len("-->")
				b.WriteByte(' ')
				continue
			}
			break
		}
		gt := strings.IndexByte(s[i:], '>')
		if gt < 0 {
			break
		}
		raw := s[i+1 : i+gt]
		opening := !strings.HasPrefix(strings.TrimSpace(raw), "/")
		name := tagName(raw)
		i += gt + 1
		if opening && (name == "script" || name == "style") {
			if end := indexFold(s[i:], "</"+name); end >= 0 {
				i += end
			} else {
				break
			}
			continue
		}
		b.WriteByte(' ')
	}
	return html.UnescapeString(b.String())
}

// indexFold は sub がASCIIのとき、s 上のバイト位置を返す。
func indexFold(s, sub string) int {
	if sub == "" || len(s) < len(sub) {
		return -1
	}
	last := len(s) - len(sub)
	for i := 0; i <= last; i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

func tagName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw[0] == '/' {
		raw = strings.TrimSpace(raw[1:])
	}
	if i := strings.IndexAny(raw, " \t\r\n/"); i >= 0 {
		raw = raw[:i]
	}
	return strings.ToLower(raw)
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func summarize(s string, limit int) string {
	s = collapseSpace(stripTags(s))
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	cut := limit
	for cut > limit-20 && cut > 0 && !unicode.IsSpace(runes[cut-1]) {
		cut--
	}
	if cut <= 0 || cut <= limit-20 {
		cut = limit
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}

// safeURL は http/https のリンクだけを返す。相対URLは base で解決する。
func safeURL(raw, base string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if base != "" {
		if b, err := url.Parse(base); err == nil {
			u = b.ResolveReference(u)
		}
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return ""
	}
	if u.Host == "" {
		return ""
	}
	return u.String()
}
