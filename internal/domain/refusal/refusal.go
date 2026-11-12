// Package refusal は、操作を断る理由を持つ。Message は利用者に見せる文言で、仕様の文言のとおりに書く。
package refusal

// Refusal は、操作を断る理由。
type Refusal struct {
	Message string
}

func (r *Refusal) Error() string { return r.Message }

// New は、message で断るエラーを返す。
func New(message string) error { return &Refusal{Message: message} }
