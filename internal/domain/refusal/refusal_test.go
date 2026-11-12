package refusal_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func TestRefusalKeepsTheMessage(t *testing.T) {
	err := fmt.Errorf("create: %w", refusal.New("タイトルを入力してください"))
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != "タイトルを入力してください" {
		t.Errorf("As = %v, %+v", errors.As(err, &r), r)
	}
	if refusal.New("x").Error() != "x" {
		t.Error("Error() is not the message")
	}
}
