package gitstore

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lock は、Store ごとのファイルロックをかける。1 回の操作のあいだ持つ。画面のサーバーと CLI が
// 同じ Store に書いても、コミットは 1 つずつ順に作られる(保存形式「技術構成」)。
// flock はファイルを開くごとに働くので、同じプロセスの中の 2 つの操作も順になる。
func lock(root string) (unlock func(), err error) {
	f, err := os.OpenFile(filepath.Join(root, ".git", "hedgehug.lock"), os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // G304: パスは Store の根と決まったファイル名から作る
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
