package search

import (
	"cmp"
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// snapshot は、表示中の 1 つの Store から読んだ、照合の材料に要るもの。ワークスペース・プロジェクト・アイテムは
// 削除済みを含む(関連・親・診断が、Store のすべてを見るため)。
type snapshot struct {
	info        port.StoreInfo
	order       int
	me          string
	retention   int
	members     []member.Member
	names       map[string]string // ログイン名を小文字にしたもの → 表示名
	whoNames    item.Names        // 履歴の文の人の表示(history.Updated)
	records     []transfer.Record
	spaces      map[workspace.ID]*space
	projects    map[project.ID]project.Project
	items       []item.Item // ワークスペースの内部 ID、プロジェクトの並び、番号の順
	byID        map[item.ID]item.Item
	shown       []item.Item // 画面に出るアイテム(削除されておらず、プロジェクトも削除されていない。app/items と同じ)
	targets     []item.Item // 材料を作るアイテム(ワークスペースの並び、プロジェクトの並び、番号の順)
	logs        map[item.ID][]change.Record
	descs       map[item.ID]string
	blocked     map[item.ID]bool
	invalid     map[item.ID]bool
	overwritten map[item.ID]bool
}

// space は、ワークスペース 1 つの材料。order は、削除されていないワークスペースのサイドバーの並びの位置。
type space struct {
	ws          workspace.Workspace
	rank        string
	order       int
	old         []string
	settings    workspace.Settings
	hasSettings bool // 設定を読めなかった(作成の行がない壊れたデータ)ときだけ偽。診断は値の判定を飛ばす
	plan        planning.Plan
	hidden      map[change.ID]bool
}

// readSnapshot は、Store 1 つを読む。説明は q.NeedsText のときだけ、診断は q.NeedsInvalid のときだけ(E1-15)。
func readSnapshot(r port.Reader, q query.Query) (*snapshot, error) {
	s := &snapshot{
		spaces: map[workspace.ID]*space{}, projects: map[project.ID]project.Project{}, byID: map[item.ID]item.Item{},
		logs: map[item.ID][]change.Record{}, descs: map[item.ID]string{},
	}
	var err error
	if s.retention, err = r.RetentionDays(); err != nil {
		return nil, err
	}
	if s.members, err = r.Members().List(); err != nil {
		return nil, err
	}
	if s.records, err = r.Transfers().List(); err != nil {
		return nil, err
	}
	s.names = map[string]string{}
	for _, m := range s.members {
		s.names[strings.ToLower(m.Login)] = m.Name
	}
	s.whoNames = history.NamesOf(s.members)
	if err := s.readWorkspaces(r); err != nil {
		return nil, err
	}
	s.pick(q.Deleted)
	if err := s.readLogs(r, q); err != nil {
		return nil, err
	}
	s.blocked = s.blockedSet()
	if q.NeedsInvalid {
		s.invalid = s.invalidSet()
	}
	return s, nil
}

// readWorkspaces は、すべてのワークスペース(削除済みを含む)と、その下のプロジェクト・アイテムを読み、
// 削除されていないワークスペースにサイドバーの並び(workspace.RankOf、同じなら内部 ID。E1-10)の位置を振る。
func (s *snapshot) readWorkspaces(r port.Reader) error {
	wss, err := r.Workspaces().List()
	if err != nil {
		return err
	}
	var live []*space
	for _, w := range wss {
		sp, err := readSpace(r, w)
		if err != nil {
			return err
		}
		s.spaces[w.ID] = sp
		if w.Deleted == nil {
			live = append(live, sp)
		}
		if err := s.readItems(r, w.ID); err != nil {
			return err
		}
	}
	slices.SortFunc(live, func(a, b *space) int { return cmp.Or(cmp.Compare(a.rank, b.rank), cmp.Compare(a.ws.ID, b.ws.ID)) })
	for i, sp := range live {
		sp.order = i
	}
	return nil
}

// readSpace は、ワークスペース w の設定とログ(並び・旧い接頭辞・計画・履歴に出さない操作)を読む。
func readSpace(r port.Reader, w workspace.Workspace) (*space, error) {
	st, stErr := r.Workspaces().Settings(w.ID)
	if stErr != nil && !errors.Is(stErr, port.ErrNotFound) {
		return nil, stErr
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
	if err != nil {
		return nil, err
	}
	return &space{
		ws: w, rank: workspace.RankOf(log), old: workspace.OldPrefixes(log), settings: st, hasSettings: stErr == nil,
		plan: planning.FromLog(log), hidden: history.HiddenOps(log),
	}, nil
}

// readItems は、ワークスペース ws のプロジェクトとアイテムを、削除済みを含めて読む。
func (s *snapshot) readItems(r port.Reader, ws workspace.ID) error {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return err
	}
	for _, p := range ps {
		s.projects[p.ID] = p
		its, err := r.Items().List(p.ID)
		if err != nil {
			return err
		}
		for _, it := range its {
			s.byID[it.ID] = it
		}
		s.items = append(s.items, its...)
	}
	return nil
}

// spaceOf は、ワークスペース id の材料。読めなかったワークスペース(壊れたデータの、ないワークスペースを指すアイテム)なら空。
func (s *snapshot) spaceOf(id workspace.ID) *space {
	if sp, ok := s.spaces[id]; ok {
		return sp
	}
	return &space{}
}

// isShown は、アイテムが画面に出るか(削除されておらず、プロジェクトも削除されていない)。
func (s *snapshot) isShown(it item.Item) bool {
	p, ok := s.projects[it.Place.Project]
	return it.Deleted == nil && ok && p.Deleted == nil
}

// pick は、画面に出るアイテムと、材料を作るアイテムを選ぶ。材料を作るのは、ワークスペースが削除されていないアイテムのうち、
// deleted なら削除済みのもの(プロジェクトは削除済みでもよい。E1-8)、でなければ画面に出るもの。
func (s *snapshot) pick(deleted bool) {
	for _, it := range s.items {
		if s.isShown(it) {
			s.shown = append(s.shown, it)
		}
		sp := s.spaces[it.Place.Workspace]
		if sp == nil || sp.ws.Deleted != nil {
			continue
		}
		if (deleted && it.Deleted != nil) || (!deleted && s.isShown(it)) {
			s.targets = append(s.targets, it)
		}
	}
	slices.SortStableFunc(s.targets, func(a, b item.Item) int {
		return cmp.Compare(s.spaces[a.Place.Workspace].order, s.spaces[b.Place.Workspace].order)
	})
}

// readLogs は、材料を作るアイテムのログ(と q.NeedsText なら説明)を読む。診断をするなら、Issue のリンクを見るため
// 削除されていないすべてのアイテムのログも読む。
func (s *snapshot) readLogs(r port.Reader, q query.Query) error {
	ids := make([]item.ID, 0, len(s.targets))
	for _, it := range s.targets {
		ids = append(ids, it.ID)
	}
	if q.NeedsInvalid {
		for _, it := range item.Live(s.items) {
			ids = append(ids, it.ID)
		}
	}
	for _, id := range ids {
		if _, ok := s.logs[id]; ok {
			continue
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		s.logs[id] = log
	}
	if !q.NeedsText {
		return nil
	}
	for _, it := range s.targets {
		d, err := r.Items().Description(it.ID)
		if err != nil {
			return err
		}
		s.descs[it.ID] = d
	}
	return nil
}
