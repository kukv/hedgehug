import { createMyTasksPort } from "@hedgehug/api";
import {
  createMyTaskBase,
  createMyTasks,
  get,
  readonly,
  type SearchRow,
  type WorkspaceSettings,
  writable,
} from "@hedgehug/core";
import { itemChanges, reloads } from "./reloads";
import { reloadCandidates, screenStates } from "./search";
import { screen, workspacesPort } from "./wire";
import { timer, toasts } from "./wireBase";

// マイタスクの画面(04「マイタスク」、E4a)の組み立て。wire.ts が 300 行なので、ここで作る。

const port = createMyTasksPort();
export const myTasks = createMyTasks({ port, screenStates, timer, toasts });
// myTaskBase は、絞り込みなしのマイタスク(空の表示の見分けと、候補の範囲に使う)。
export const myTaskBase = createMyTaskBase({ port });

// 行のワークスペースの設定(ステータスのボタンに使う)。ワークスペースごとに 1 回だけ読み、失敗したら次に読み直す。
// settings は読み込み(飛んでいるものも)、loaded は読めたもの。読み直すときは settings だけを捨て、読めるまでは前の設定で
// 描く(ステータスのメニューを開いたまま閉じないように)。round は捨てた回数で、捨てる前に始めた読み込みの結果は入れない。
let settings = new Map<string, Promise<WorkspaceSettings>>();
const loaded = new Map<string, WorkspaceSettings>();
let round = 0;
const settingsLoaded = writable(0);
// myTaskSettingsLoaded は、設定を読み終えるたびに増える(行が描き直すため)。
export const myTaskSettingsLoaded = readonly(settingsLoaded);

const keyOf = (storeId: string, workspaceId: string) => `${storeId}/${workspaceId}`;

// myTaskSettings は、読めた行のワークスペースの設定。まだ読めていなければ undefined。
export const myTaskSettings = (storeId: string, workspaceId: string) =>
  loaded.get(keyOf(storeId, workspaceId));

function loadSettings(rows: SearchRow[]) {
  for (const r of rows) {
    const key = keyOf(r.storeId, r.workspaceId);
    if (settings.has(key)) continue;
    const mine = round;
    const p = workspacesPort.get(r.storeId, r.workspaceId).then((d) => d.settings);
    settings.set(key, p);
    const map = settings;
    p.then(
      (s) => {
        if (mine !== round) return;
        loaded.set(key, s);
        settingsLoaded.update((n) => n + 1);
      },
      () => {
        if (map.get(key) === p) map.delete(key);
      },
    );
  }
}

myTasks.state.subscribe((s) => loadSettings(s.rows));

// Store が変わった知らせとアイテムを変えたあとに読み直す。絞り込みなしの行はいつも、画面の行と絞り込み欄の候補の元は、
// マイタスクの画面を出しているときだけ読み直す(検索の reloads と同じ見分け)。解決できないアドレスもマイタスクを出すので、
// アドレスではなく出している画面で見る。行のワークスペースの設定は、いつも捨てる(画面を開くと読み直す)。
async function refresh() {
  round += 1;
  settings = new Map();
  if (get(screen.current)?.name !== "mytasks") return myTaskBase.reload();
  await Promise.all([myTaskBase.reload(), myTasks.reload(), reloadCandidates()]);
}
reloads.add(refresh);
itemChanges.add(refresh);
