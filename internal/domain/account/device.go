package account

import "time"

// RefreshBefore は、期限付きのトークンを、期限のどれだけ前から取り直すか(前回の実装と同じ)。
const RefreshBefore = 5 * time.Minute

// DeviceToken は、デバイスフローで得たトークンの組。OS のキーチェーンにだけ置く(07「デバイスフロー」)。
type DeviceToken struct {
	Access           string
	Refresh          string    // 更新用のトークン。期限のないトークンでは空のことがある
	ExpiresAt        time.Time // Access の期限。ゼロなら期限なし(expires_in が 0)
	RefreshExpiresAt time.Time // Refresh の期限。ゼロなら分からない
}

// Fresh は、まだ取り直さなくてよいか(期限がないか、期限まで RefreshBefore より長い)。
func (t DeviceToken) Fresh(now time.Time) bool {
	return t.ExpiresAt.IsZero() || now.Add(RefreshBefore).Before(t.ExpiresAt)
}

// RefreshExpired は、更新用のトークンの期限が過ぎたか。
func (t DeviceToken) RefreshExpired(now time.Time) bool {
	return !t.RefreshExpiresAt.IsZero() && !now.Before(t.RefreshExpiresAt)
}
