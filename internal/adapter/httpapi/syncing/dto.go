package syncing

// SyncStoreStatus は、remote がある 1 つの Store の同期の状態(06「同期の状態の表示」)。
type SyncStoreStatus struct {
	Store       string `json:"store"`
	Name        string `json:"name"`
	Hidden      bool   `json:"hidden"`
	ReadOnly    bool   `json:"readOnly"`
	Error       string `json:"error" doc:"同期エラーなら短い理由(06 の表の文言)。なければ空"`
	Unsent      int    `json:"unsent" doc:"未送信の保存の数"`
	LastSuccess string `json:"lastSuccess" doc:"最後に成功した同期の時刻(RFC 3339、UTC)。なければ空"`
}

// SyncStatusList は、同期の状態の一覧(サイドバーの並び。非表示の Store も含む)と、自動同期のオン・オフ。
type SyncStatusList struct {
	Auto   bool              `json:"auto" doc:"「裏で自動的に同期する」がオンか(オフラインの帯の文を選ぶ)"`
	Stores []SyncStoreStatus `json:"stores" nullable:"false"`
}

// SyncStoreResult は、1 つの Store を手動で同期した結果(決定 C2-21)。
type SyncStoreResult struct {
	Result        string `json:"result" enum:"synced,skipped,failed" doc:"synced はトースト、skipped は案内、failed はエラーのトースト"`
	Message       string `json:"message"`
	MirrorWarning string `json:"mirrorWarning" doc:"ミラーに送れなかったときの警告のトースト。なければ空"`
}

// SyncAllResult は、「すべて同期」の結果。Lines は 1 つのトーストの行。
type SyncAllResult struct {
	Result string   `json:"result" enum:"synced,failed,nothing"`
	Lines  []string `json:"lines" nullable:"false"`
}

// SyncHold は、見えているタブが、自動の同期を待たせる条件にあるかの知らせ(決定 C2-18)。
type SyncHold struct {
	Tab  string `json:"tab" minLength:"1" maxLength:"64"`
	Hold bool   `json:"hold"`
}

// SyncNetwork は、タブがネットワークにつながっているかの知らせ(決定 C2-18b)。
type SyncNetwork struct {
	Tab    string `json:"tab" minLength:"1" maxLength:"64"`
	Online bool   `json:"online"`
}
