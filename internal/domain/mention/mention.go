// Package mention は、コメントの本文のメンション(@ログイン名)を見つける。
package mention

import "strings"

// Has は、body に login へのメンションがあるか。`@` のあとにログイン名が続き、その直後がログイン名に使える
// 文字(member.ValidLogin の文字)でなければメンション。大文字と小文字を区別しない。`@` の前の文字は問わない。
func Has(body, login string) bool {
	if login == "" {
		return false
	}
	for i := 0; i < len(body); i++ {
		if body[i] != '@' {
			continue
		}
		rest := body[i+1:]
		if len(rest) < len(login) || !strings.EqualFold(rest[:len(login)], login) {
			continue
		}
		if len(rest) == len(login) || !loginChar(rest[len(login)]) {
			return true
		}
	}
	return false
}

// Any は、body に誰かへのメンションがあるか。`@` の直後にログイン名に使える文字が 1 文字以上続けばよい。
// `@` の前の文字とコードの中かは問わない(Has と同じ読み方)。
func Any(body string) bool {
	for i := 0; i+1 < len(body); i++ {
		if body[i] == '@' && loginChar(body[i+1]) {
			return true
		}
	}
	return false
}

func loginChar(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-'
}
