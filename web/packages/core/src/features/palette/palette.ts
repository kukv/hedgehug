import type { Item } from "../../model/item";
import type { PaletteIndex, RecentItem } from "../../model/palette";
import type { ItemsPort } from "../../port/items";
import type { PalettePort } from "../../port/palette";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { StoreNode } from "../catalog/catalog";
import type { ValueOption } from "../items/properties";
import { statusOptions } from "../items/properties";
import { itemDisplayId, type Route } from "../router/route";
import {
  type ActionDeps,
  commandCandidates,
  createCandidates,
  selectedCandidates,
  settingsCandidates,
} from "./actions";
import { buildRows, type Candidate, noMatch, type Row } from "./candidates";
import {
  itemCandidates,
  projectCandidates,
  recentCandidates,
  workspaceCandidates,
} from "./destinations";
import { byName, matchItems, matchProjects, matchWorkspaces } from "./match";
import type { SelectedItem, Selection } from "./selection";

// コマンドパレットの状態(02「コマンドパレット」、A5c-2・A5c-9・A5c-15・A5c-16)。

export interface PaletteState {
  input: string;
  rows: Row[];
  // selected は、選んでいる行の位置。押せる行がないときは -1。
  selected: number;
  // mode は、候補の一覧("main")か、ステータスの一覧({ status })か。
  mode: "main" | { status: SelectedItem };
  footer: string;
  // selectedHead は、選んでいるアイテムの群の見出し(rows の同じ文の head の行)。storeColor は、同じ ID の
  // アイテムが別の Store にもあるとき頭に添える点の色(Store の色。A5c-9)。群を出さないときは null。
  selectedHead: { text: string; storeColor?: string } | null;
}

export interface Palette {
  state: Readable<PaletteState | null>;
  // open は、パレットを開き、索引・最近開いたもの・選んでいるアイテムを読む(読み終わると返す)。
  open(): Promise<void>;
  close(): void;
  input(text: string): void;
  move(dir: "up" | "down"): void;
  // choose は、index の行(省くと選んでいる行)を実行する。newTab(Ctrl+Enter)は、行き先のある候補を新しいタブで開く。
  choose(index?: number, newTab?: boolean): Promise<void>;
}

export interface PaletteDeps extends Omit<ActionDeps, "close"> {
  port: Pick<PalettePort, "index" | "recent">;
  items: Pick<ItemsPort, "get">;
  workspaces: Pick<WorkspacesPort, "get">;
  selection: Pick<Selection, "current">;
  // route は、いまの画面。変わったら閉じる(A5c-16)。
  route: Readable<Route>;
  openTab(route: Route): void;
  now(): Date;
}

export function paletteFooter(mac: boolean): string {
  const mod = mac ? "Cmd" : "Ctrl";
  return `↑↓ で選択 ・ Enter で開く ・ ${mod}+Enter で新しいタブ ・ Esc で閉じる`;
}

// Picked は、選んでいるアイテム(開いた時点に読んだもの)。
interface Picked {
  sel: SelectedItem;
  item: Item;
  store: StoreNode;
  display: string;
}

// Session は、開いているあいだの中身。開き直すと作り直す。
interface Session {
  input: string;
  index: PaletteIndex | null;
  recent: RecentItem[];
  picked: Picked | null;
  // options は、ステータスの一覧に切り替えたときの項目。
  options: ValueOption[] | null;
}

const selectable = (r: Row | undefined): r is Extract<Row, { kind: "candidate" }> =>
  r?.kind === "candidate" && !r.c.disabled;

export function createPalette(deps: PaletteDeps): Palette {
  const state = writable<PaletteState | null>(null);
  let session: Session | null = null;

  function close() {
    session = null;
    state.set(null);
  }
  const a: ActionDeps = { ...deps, close };

  function statusRows(s: Session, picked: Picked, options: ValueOption[]): Row[] {
    const list: Candidate[] = options.map((o) => ({
      key: `status:${o.id}`,
      group: "selected",
      label: o.label,
      checked: o.checked,
      ...(o.disabled ? { disabled: true, reason: o.reason, hint: o.reason } : {}),
      ...(!o.disabled && o.hint ? { hint: o.hint } : {}),
      run: async () => {
        const p = picked.sel.setStatus(o.id);
        close();
        await p;
      },
    }));
    const shown = byName(list, s.input);
    return [
      { kind: "head", text: `${picked.display} のステータス` },
      ...(shown.length > 0
        ? shown.map((c): Row => ({ kind: "candidate", c }))
        : [{ kind: "more", text: noMatch } as const]),
    ];
  }

  function mainRows(s: Session, head: string | undefined): Row[] {
    const stores = get(deps.catalog.stores);
    const now = deps.now();
    const p = s.picked;
    const toStatus = () => toStatusMode(s);
    return buildRows(
      {
        command: byName(commandCandidates(a), s.input),
        selected: p
          ? byName(selectedCandidates(a, p.item, p.store.readOnly, toStatus), s.input)
          : [],
        create: byName(createCandidates(a), s.input),
        settings: byName(settingsCandidates(a), s.input),
        recent: recentCandidates(a, s.recent),
        item: s.index ? itemCandidates(a, matchItems(s.index, stores, s.input, now)) : [],
        project: s.index ? projectCandidates(a, matchProjects(s.index, stores, s.input, now)) : [],
        workspace: workspaceCandidates(a, matchWorkspaces(stores, s.input)),
      },
      s.input,
      head === undefined ? {} : { selected: head },
    );
  }

  // render は、行を作り直す。keep なら、選んでいた候補が残っていればそれを選んだまま(読み終わりで行が増えたとき)。
  function render(keep: boolean) {
    const s = session;
    if (!s) return;
    const p = s.picked;
    const head = p ? `${p.display} に対して` : undefined;
    const rows = p && s.options ? statusRows(s, p, s.options) : mainRows(s, head);
    const prev = get(state);
    const was = keep && prev ? prev.rows[prev.selected] : undefined;
    const kept = selectable(was)
      ? rows.findIndex((r) => selectable(r) && r.c.key === was.c.key)
      : -1;
    const dot =
      p && s.index?.items.some((i) => i.display === p.display && i.storeId !== p.store.id);
    const showsHead =
      head !== undefined && !s.options && rows.some((r) => r.kind === "head" && r.text === head);
    state.set({
      input: s.input,
      rows,
      selected: kept >= 0 ? kept : rows.findIndex((r) => selectable(r)),
      mode: p && s.options ? { status: p.sel } : "main",
      footer: paletteFooter(deps.mac),
      selectedHead: showsHead
        ? { text: head, ...(dot ? { storeColor: p.store.color } : {}) }
        : null,
    });
  }

  // toStatusMode は、ステータスの一覧に切り替える(入力は空に戻す。A5c-15)。設定を読めなければ切り替えない。
  async function toStatusMode(s: Session) {
    const p = s.picked;
    if (!p) return;
    const detail = await deps.workspaces.get(p.item.storeId, p.item.workspaceId).catch(() => null);
    if (!detail || session !== s) return;
    s.options = statusOptions(detail.settings, p.item.status, true, p.item.projectId);
    s.input = "";
    render(false);
  }

  // readPicked は、選んでいるアイテムを読む(種類・ステータス・閲覧のみを知るため)。一覧にない Store のものは null。
  async function readPicked(sel: SelectedItem): Promise<Picked | null> {
    const item = await deps.items.get(sel.storeId, sel.itemId);
    const store = get(deps.catalog.stores).find((x) => x.id === item.storeId);
    const ws = store?.workspaces.find((w) => w.id === item.workspaceId);
    if (!store || !ws) return null;
    return { sel, item, store, display: itemDisplayId(ws.prefix, item.number) };
  }

  // load は、読み終わったら中身に入れて作り直す(閉じた・開き直したあとは、いまの中身を作り直すだけで変わらない)。
  // 読めなければその群を出さない(A5c-2)。
  const load = <T>(read: Promise<T>, put: (v: T) => void) =>
    read.then(
      (v) => {
        put(v);
        render(true);
      },
      () => {},
    );

  // 画面が変わったら閉じる(subscribe がすぐ渡すいまの値では、まだ開いていないので何も起きない)。
  deps.route.subscribe(() => close());

  return {
    state: readonly(state),
    async open() {
      const s: Session = { input: "", index: null, recent: [], picked: null, options: null };
      session = s;
      render(false);
      const sel = deps.selection.current();
      await Promise.all([
        load(deps.port.index(), (v) => {
          s.index = v;
        }),
        load(deps.port.recent(), (v) => {
          s.recent = v;
        }),
        sel
          ? load(readPicked(sel), (v) => {
              s.picked = v;
            })
          : null,
      ]);
    },
    close,
    input(text) {
      if (!session) return;
      session.input = text;
      render(false);
    },
    move(dir) {
      const st = get(state);
      if (!st) return;
      const step = dir === "up" ? -1 : 1;
      for (let i = st.selected + step; i >= 0 && i < st.rows.length; i += step) {
        if (selectable(st.rows[i])) return state.set({ ...st, selected: i });
      }
    },
    async choose(index, newTab = false) {
      const st = get(state);
      if (!st) return;
      const row = st.rows[index ?? st.selected];
      if (!selectable(row)) return;
      if (newTab && row.c.route) {
        close();
        deps.openTab(row.c.route);
        return;
      }
      await row.c.run();
    },
  };
}
