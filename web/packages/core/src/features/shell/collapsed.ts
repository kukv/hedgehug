import type { ScreenStates } from "../screenstate/screenStates";

// サイドバーで、プロジェクトの行を隠しているワークスペースの内部 ID(G1-14)。
// 画面の状態の画面 `sidebar` に `{"collapsed": [ID…]}` で残す。
export interface Collapsed {
  has(id: string): boolean;
  toggle(id: string): void;
  // ready は、残していた値を読み終えたとき(読めなくても)に解ける。
  ready: Promise<void>;
}

const screen = "sidebar";

interface Saved {
  collapsed: string[];
}

function idsOf(saved: unknown): string[] {
  const list = (saved as Saved | null)?.collapsed;
  if (!Array.isArray(list) || !list.every((id) => typeof id === "string")) return [];
  return list;
}

export function createCollapsed(states: ScreenStates): Collapsed {
  const ids = new Set<string>();
  // 読み終える前の切り替え。読み終えたあとに、読んだ値へ順に当てて書く(失わない)。
  const early: string[] = [];
  let loaded = false;

  const flip = (id: string) => {
    if (!ids.delete(id)) ids.add(id);
  };
  const save = () => states.write<Saved>(screen, { collapsed: [...ids] });

  const ready = states.read<unknown>(screen, null).then((saved) => {
    ids.clear();
    for (const id of idsOf(saved)) ids.add(id);
    for (const id of early) flip(id);
    loaded = true;
    if (early.length > 0) save();
  });

  return {
    has: (id) => ids.has(id),
    toggle(id) {
      flip(id);
      if (loaded) save();
      else early.push(id);
    },
    ready,
  };
}
