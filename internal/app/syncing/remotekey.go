package syncing

import "strings"

// remoteKey は、remote が同じかを比べる形(決定 C2-11)。方式・利用者名・末尾の .git と / を除き、
// git@host:path を host/path にし、小文字にする。SSH と HTTPS の書き方の違いでは変わらない。
func remoteKey(raw string) string {
	s := strings.TrimSpace(raw)
	if _, rest, ok := strings.Cut(s, "://"); ok {
		host, path, _ := strings.Cut(rest, "/")
		if i := strings.LastIndex(host, "@"); i >= 0 {
			host = host[i+1:]
		}
		s = host + "/" + path
	} else if i := strings.Index(s, "@"); i >= 0 && !strings.Contains(s[:i], "/") {
		s = strings.Replace(s[i+1:], ":", "/", 1)
	}
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.ToLower(s)
}
