import type { SearchPort } from "../../port/search";
import { readOnlyNote } from "../catalog/readonly";
import { hasKeyTerm } from "../search/caret";
import type { Timer } from "../toasts/toasts";
import type { Candidate } from "./candidates";

// 「このクエリで」の群(02「コマンドパレット」、E1-21)。キーの形の語がある入力だけを、150ms 待ってサーバーで解析し、
// いまの入力の応答が「キーが 1 つ以上あってエラーがない」ときだけ「検索で開く」を出す。キーの形の語がなければ、
// サーバーの答えも「キーなし」なので聞かない。

export const queryWait = 150;

export interface QueryGroupDeps {
  search: Pick<SearchPort, "search">;
  timer: Timer;
  // openSearch は、検索の画面の絞り込みを入れ替えて /search を開き、移れたかを返す。
  openSearch(query: string): Promise<boolean>;
  // bulkChangeAll は、検索をテーブルで開き、すべての行を選んで、一括変更のダイアログを開く(E2a-19)。
  bulkChangeAll(query: string): Promise<void>;
  // close は、パレットを閉じる。changed は、応答が着いて群が変わったとき。
  close(): void;
  changed(): void;
}

export interface QueryGroup {
  // input は、入力が変わったとき。前の待ちと応答を捨てて、待ち直す。
  input(text: string): void;
  // candidates は、いまの入力の応答があって出せるときの「検索で開く」。
  candidates(): Candidate[];
  // reset は、パレットを開く・閉じるとき。待ちと応答を捨てる。
  reset(): void;
}

export function createQueryGroup(deps: QueryGroupDeps): QueryGroup {
  let pending: unknown;
  let current = "";
  // shown は、current の応答で出せるときの件数・`is:deleted`・すべて閲覧のみ。
  let shown: { count: number; deleted: boolean; readOnly: boolean } | null = null;

  function reset() {
    if (pending !== undefined) deps.timer.clear(pending);
    pending = undefined;
    current = "";
    shown = null;
  }

  return {
    reset,
    input(text) {
      reset();
      if (!hasKeyTerm(text)) return;
      current = text;
      pending = deps.timer.set(() => {
        pending = undefined;
        deps.search.search(text).then(
          (r) => {
            // 入力が変わった・閉じたあとの応答は捨てる。
            if (current !== text) return;
            shown =
              r.hasKey && r.errors.length === 0
                ? {
                    count: r.rows.length,
                    deleted: r.deleted,
                    readOnly: r.rows.length > 0 && r.rows.every((x) => x.readOnly),
                  }
                : null;
            deps.changed();
          },
          // 検索の要求が届かなければ(通信の失敗)、群を出さないだけ(入力はそのまま続けられる)。
          () => {},
        );
      }, queryWait);
    },
    candidates() {
      if (shown === null) return [];
      const query = current;
      const reason = shown.deleted
        ? "削除済みのアイテムは変更できません"
        : shown.count === 0
          ? "一致するアイテムがありません"
          : shown.readOnly
            ? readOnlyNote
            : "";
      return [
        {
          key: "query:open",
          group: "query",
          label: "検索で開く",
          hint: `${shown.count} 件`,
          run: async () => {
            deps.close();
            await deps.openSearch(query);
          },
        },
        {
          key: "query:changeAll",
          group: "query",
          label: `${shown.count} 件をまとめて変更…`,
          ...(reason ? { disabled: true, reason, hint: reason } : {}),
          run: async () => {
            deps.close();
            await deps.bulkChangeAll(query);
          },
        },
      ];
    },
  };
}
