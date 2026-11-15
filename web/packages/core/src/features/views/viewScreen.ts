import { get, type Readable } from "../../reactive/store";
import type { Search } from "../search/layout";
import type { Toasts } from "../toasts/toasts";
import { type ViewContent, viewContent, viewDirty } from "./content";
import type { Views } from "./views";

// ビューの画面の「未保存の変更」の印・「変更を破棄」・「保存」(04「保存したビュー」、E3b-4・E3b-9・E3b-12)。

export interface ViewScreen {
  // dirty は、画面の状態を読み終えたあと(ready)、一覧のそのビューの保存した内容と画面の内容が違うとき。
  // 一覧の保存した内容が替わっても画面の内容は書き換えず、比べる相手だけが替わる(E3b-17)。
  dirty: Readable<boolean>;
  // discard は、画面の内容を保存した内容に戻す(ツリーの開閉はそのまま。確認はしない)。
  discard(): void;
  // save は、画面の内容を保存し、サーバーが返した内容で画面の内容を置き換えてからトーストを出す(E3b-9)。
  // 応答を待つあいだの save は無視する。失敗したら(エラーのトーストは views が出す)画面はそのまま。
  save(): Promise<void>;
}

export interface ViewScreenDeps {
  id: string;
  views: Pick<Views, "list" | "save">;
  search: Pick<Search, "state" | "restore">;
  ready: Readable<boolean>;
  toasts: Toasts;
}

export function createViewScreen(deps: ViewScreenDeps): ViewScreen {
  // saving は、保存の応答を待っているあいだ。そのあいだの save は無視する(PUT とトーストを 2 回出さない)。
  let saving = false;
  const savedContent = (): ViewContent | null =>
    get(deps.views.list).find((v) => v.id === deps.id)?.content ?? null;

  const compute = () => {
    const c = savedContent();
    return get(deps.ready) && c !== null && viewDirty(c, get(deps.search.state));
  };

  // dirty は、購読しているあいだだけ元の 3 つを購読し、値が替わったときだけ知らせる。
  const dirty: Readable<boolean> = {
    subscribe(run) {
      let last: boolean | undefined;
      const emit = () => {
        const next = compute();
        if (next === last) return;
        last = next;
        run(next);
      };
      const stops = [
        deps.ready.subscribe(emit),
        deps.views.list.subscribe(emit),
        deps.search.state.subscribe(emit),
      ];
      return () => {
        for (const stop of stops) stop();
      };
    },
  };

  return {
    dirty,
    discard() {
      const c = savedContent();
      if (c) deps.search.restore(c);
    },
    async save() {
      if (saving) return;
      saving = true;
      // views.save は投げない(失敗は null)。
      const v = await deps.views.save(deps.id, viewContent(get(deps.search.state)));
      saving = false;
      if (!v) return;
      deps.search.restore(v.content);
      deps.toasts.show("ビューを保存しました");
    },
  };
}
