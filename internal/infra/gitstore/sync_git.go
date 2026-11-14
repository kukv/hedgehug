package gitstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/infra/gitstore/saves"
)

// remotePrefix は、受け取った remote のブランチを置く場所。利用者の git の remote の設定には触らない。
const remotePrefix = "refs/hedgehug/remote/"

// remoteName は、go-git の中だけで使う remote の名前。利用者の git の設定には書かない。
const remoteName = "hedgehug"

// errRejected は、送信が他の人の更新と重なって拒否されたこと。Sync が受け取りからやり直す。
var errRejected = errors.New("rejected: the remote moved")

type link struct {
	remote *git.Remote
	auth   transport.AuthMethod
}

func newLink(repo *git.Repository, rm port.Remote) link {
	r := git.NewRemote(repo.Storer, &config.RemoteConfig{Name: remoteName, URLs: []string{rm.URL}})
	var auth transport.AuthMethod
	if rm.Token != "" {
		auth = &githttp.BasicAuth{Username: "x-access-token", Password: rm.Token}
	}
	return link{remote: r, auth: auth}
}

// fetch は、remote の全ブランチを受け取り、branch と同じ名前のブランチの先端を返す。
// remote が空なら、ゼロのハッシュと nil。remote が空でなく、そのブランチがなければ port.ErrNoCommonHistory。
func (l link) fetch(ctx context.Context, repo *git.Repository, branch plumbing.ReferenceName) (plumbing.Hash, error) {
	return l.fetchTo(ctx, repo, branch, remotePrefix)
}

// fetchTo は、fetch と同じことを、受け取ったブランチを prefix の下に置いて行う(ミラーは別の場所に置く)。
func (l link) fetchTo(ctx context.Context, repo *git.Repository, branch plumbing.ReferenceName, prefix string) (plumbing.Hash, error) {
	wctx, done := watch(ctx)
	defer done()
	err := l.remote.FetchContext(wctx, &git.FetchOptions{
		RefSpecs:   []config.RefSpec{config.RefSpec("+refs/heads/*:" + prefix + "*")},
		RemoteName: remoteName,
		Auth:       l.auth,
		Force:      true,
		// remote で消えたブランチは手元からも消す。消えた main の古い先端と合わせない(決定 4)。
		Prune: true,
		Tags:  git.NoTags,
	})
	switch {
	case errors.Is(err, transport.ErrEmptyRemoteRepository):
		return plumbing.ZeroHash, nil
	case err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate):
		return plumbing.ZeroHash, classify(wctx, err, false)
	}
	ref, err := repo.Reference(plumbing.ReferenceName(prefix+branch.Short()), true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, port.ErrNoCommonHistory
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}

// send は、branch を remote の同じ名前のブランチへ送る。履歴を書き換えない(強制しない)。
// theirs は、この回で合わせた remote の先端(remote が空ならゼロ)、mine は送る自分の先端。
func (l link) send(ctx context.Context, branch plumbing.ReferenceName, theirs, mine plumbing.Hash) error {
	wctx, done := watch(ctx)
	defer done()
	spec := config.RefSpec(branch.String() + ":" + branch.String())
	err := l.remote.PushContext(wctx, &git.PushOptions{RemoteName: remoteName, RefSpecs: []config.RefSpec{spec}, Auth: l.auth})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	// go-git が送る前に見つける重なりは "non-fast-forward update" の文で返る。
	if strings.Contains(err.Error(), "non-fast-forward") {
		return errRejected
	}
	// サーバーが断る文は、サーバーごとに違う(git の "failed to update ref"、GitHub の "cannot lock ref" など)。
	// 文で見分けず、remote の先端をもう一度見る。合わせた先端から動いていれば、他の人の更新と重なったので、
	// 受け取りからやり直す。動いていなければ、重なりではない失敗としてそのまま返す。
	// remote の先端が自分の先端なら、サーバーは更新したが応答が届かなかったので、送れたものとする
	// (やり直すと、次の回で送った数が 0 になる)。
	tip, lerr := l.tip(ctx, branch)
	if lerr != nil {
		return errors.Join(classify(wctx, err, true), lerr)
	}
	if tip == mine {
		return nil
	}
	if tip != theirs {
		return errRejected
	}
	return refused(wctx, err)
}

// tip は、remote の branch の先端を、受け取らずに見る。remote が空か、そのブランチがなければゼロ。
func (l link) tip(ctx context.Context, branch plumbing.ReferenceName) (plumbing.Hash, error) {
	wctx, done := watch(ctx)
	defer done()
	refs, err := l.remote.ListContext(wctx, &git.ListOptions{Auth: l.auth})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return plumbing.ZeroHash, nil
	}
	if err != nil {
		return plumbing.ZeroHash, classify(wctx, err, false)
	}
	for _, r := range refs {
		if r.Name() == branch {
			return r.Hash(), nil
		}
	}
	return plumbing.ZeroHash, nil
}

// checkRemoteStore は、remote の先端が hedgehug の Store で、この hedgehug が読める形式かを確かめる。
func checkRemoteStore(repo *git.Repository, h plumbing.Hash) error {
	c, err := repo.CommitObject(h)
	if err != nil {
		return err
	}
	f, err := c.File(storeFile)
	if errors.Is(err, object.ErrFileNotFound) {
		return port.ErrRemoteNotStore
	}
	if err != nil {
		return err
	}
	text, err := f.Contents()
	if err != nil {
		return err
	}
	var y storeYAML
	if yaml.Unmarshal([]byte(text), &y) != nil || y.Store == "" || y.Format == 0 {
		return port.ErrRemoteNotStore
	}
	if y.Format > Format {
		return port.ErrRemoteNewerFormat
	}
	return nil
}

// readTree は、コミットのファイルをすべて読む。ゼロのハッシュなら空。
func readTree(repo *git.Repository, h plumbing.Hash) (tree, error) {
	out := tree{}
	if h.IsZero() {
		return out, nil
	}
	c, err := repo.CommitObject(h)
	if err != nil {
		return nil, err
	}
	files, err := c.Files()
	if err != nil {
		return nil, err
	}
	err = files.ForEach(func(f *object.File) error {
		r, err := f.Reader()
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		b, err := io.ReadAll(r)
		out[f.Name] = b
		return err
	})
	return out, err
}

// treeChanges は、writeTree が作業ツリーで変えたファイル(Store の根からの相対パス)。
// written は書いたもの、removed は消したもの、created は written のうち新しく作ったもの(失敗したら消すため)。
type treeChanges struct {
	written, removed, created []string
}

// writeTree は、作業ツリーを from から to の中身にする。from と to にないファイル(OS やエディタが作るもの)には触らない。
func writeTree(root string, from, to tree) (treeChanges, error) {
	var c treeChanges
	removed, err := removeGone(root, from, to)
	c.removed = removed
	if err != nil {
		return c, err
	}
	for _, p := range sortedKeys(to) {
		old, existed := from[p]
		if existed && string(old) == string(to[p]) {
			continue
		}
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { //nolint:gosec // G301: Store のファイルは共有するもので秘密ではない
			return c, err
		}
		if !existed {
			c.created = append(c.created, p)
		}
		c.written = append(c.written, p)
		if err := os.WriteFile(full, to[p], 0o644); err != nil { //nolint:gosec // G306: Store のファイルは共有するもので秘密ではない
			return c, err
		}
	}
	return c, nil
}

// removeGone は、from にあって to にないファイルを消し、消したものを返す。
func removeGone(root string, from, to tree) ([]string, error) {
	var removed []string
	for _, p := range sortedKeys(from) {
		if _, keep := to[p]; keep {
			continue
		}
		if err := os.Remove(filepath.Join(root, p)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, p)
		_ = os.Remove(filepath.Dir(filepath.Join(root, p))) // 空になったフォルダだけが消える
	}
	return removed, nil
}

// stage は、writeTree が変えたファイルだけをインデックスに載せる。作業ツリー全体は足さない(Store のものでない
// ファイルを、合わせたコミットに入れて送らないため)。
func stage(wt *git.Worktree, c treeChanges) error {
	for _, p := range c.removed {
		if _, err := wt.Remove(p); err != nil {
			return err
		}
	}
	for _, p := range c.written {
		if err := wt.AddWithOptions(&git.AddOptions{Path: p, SkipStatus: true}); err != nil {
			return err
		}
	}
	return nil
}

// countSaves は、from から辿れて exclude から辿れない保存(Hedgehug-Op を持つコミット)の数。
func countSaves(repo *git.Repository, from, exclude plumbing.Hash) (int, error) {
	if from.IsZero() {
		return 0, nil
	}
	seen := map[plumbing.Hash]bool{}
	if !exclude.IsZero() {
		if err := walk(repo, exclude, func(c *object.Commit) { seen[c.Hash] = true }); err != nil {
			return 0, err
		}
	}
	n := 0
	err := walk(repo, from, func(c *object.Commit) {
		if !seen[c.Hash] && strings.Contains(c.Message, saves.OpTrailer) {
			n++
		}
	})
	return n, err
}

func walk(repo *git.Repository, from plumbing.Hash, fn func(c *object.Commit)) error {
	iter, err := repo.Log(&git.LogOptions{From: from})
	if err != nil {
		return err
	}
	return iter.ForEach(func(c *object.Commit) error {
		fn(c)
		return nil
	})
}
