import { createAccountsPort, createLinksPort } from "@hedgehug/api";
import { createLinkDialog, createLinkList, createProjectLinks } from "@hedgehug/core";
import { reloads } from "./reloads";
import { catalog, detail, navigation, toasts } from "./wire";

// GitHub の欄(07「PR と Issue のリンク」)。wire.ts が 300 行なので、ここで作る。
const port = createLinksPort();
export const projectLinks = createProjectLinks({ links: port });
export const linkList = createLinkList({
  links: port,
  accounts: createAccountsPort(),
  toasts,
  onChanged: () => void projectLinks.reload(),
});
// リンクしたら、一覧と、プロジェクトのリポジトリ(「プロジェクトのリポジトリにも追加する」で増え、次に開く
// ダイアログの選択肢になる)を読み直す。projectEditor.ts と同じく、カタログを読んでから詳細パネルを読み直す。
// 読み直す前に、覚えた取り直しの失敗を消す(付け直した行に前の失敗を残さない)。
export const linkDialog = createLinkDialog({
  links: port,
  onAdded: () => {
    linkList.forgetAll();
    void Promise.all([
      linkList.reload(),
      projectLinks.reload(),
      catalog.load().then(() => detail.reload()),
    ]);
  },
});
reloads.add(() => linkList.reload());
reloads.add(() => projectLinks.reload());
// 画面を移ったら、ダイアログを閉じる(01「画面の移り方」。前のアイテム宛てのダイアログを残さない)。
navigation.route.subscribe(() => linkDialog.close());
