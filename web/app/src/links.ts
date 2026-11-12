import { newTabUrl } from "@hedgehug/api";
import { formatRoute, type Navigation, type Route } from "@hedgehug/core";

// 画面を移る部品(サイドバーの項目・パンくず・タブ・行のタイトル)のリンク。
// <a href={links.href(r)} onclick={(e) => links.click(e, r)} onauxclick={(e) => links.click(e, r)}> で使う。
export interface Links {
  // href は、リンクのアドレス。トークンを入れない(右クリックの「新しいタブで開く」はトークンのない URL。08)。
  href(route: Route): string;
  // click は、ふつうのクリックで画面を移り、Ctrl・Cmd・Shift+クリックと中クリックで新しいタブに開く(01)。
  click(event: MouseEvent, route: Route): void;
}

export function createLinks(
  navigation: Navigation,
  token: string,
  open: (url: string) => void = (url) => window.open(url, "_blank", "noopener"),
): Links {
  return {
    href: formatRoute,
    click(event, route) {
      // 右クリック(button 2)などは、ブラウザのメニューに任せる。
      if (event.button > 1) return;
      event.preventDefault();
      if (event.button === 1 || event.ctrlKey || event.metaKey || event.shiftKey) {
        open(newTabUrl(formatRoute(route), token));
        return;
      }
      navigation.go(route);
    },
  };
}
