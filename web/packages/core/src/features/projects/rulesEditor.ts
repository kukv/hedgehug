import { Refused } from "../../model/errors";
import type { Toasts } from "../toasts/toasts";

// プロジェクト別のルールの編集(03「プロジェクト別のルール」)。検査と断りの文言はサーバーが持つので、
// ここは、断りをエラーのトーストで出すことと、保存のあとの読み直しだけを決める。

export interface RulesEditor {
  // run は、ルールの操作を 1 回行う。断られたらその文言をエラーのトーストに出し、成功したら ok があればそれを
  // トーストに出す。読み直しは、成功でも断りでもいつも行う(画面のチェックを保存済みの値に戻すため)。成功なら
  // true。断り以外のエラーは読み直したあとそのまま投げる。
  run(op: () => Promise<unknown>, ok?: string): Promise<boolean>;
}

export interface RulesEditorDeps {
  toasts: Toasts;
  // onChanged は、操作のあと(一覧と、開いているプロジェクトの読み直し)。
  onChanged(): Promise<void>;
}

export function createRulesEditor(deps: RulesEditorDeps): RulesEditor {
  return {
    async run(op, ok) {
      let done = false;
      try {
        await op();
        done = true;
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        deps.toasts.show(e.message, { error: true });
      } finally {
        await deps.onChanged();
      }
      if (done && ok !== undefined) deps.toasts.show(ok);
      return done;
    },
  };
}
