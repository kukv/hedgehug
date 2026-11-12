import type { HistoryPort } from "@hedgehug/core";

// browserHistory は、core の履歴の口を History API で実装する。
export function browserHistory(): HistoryPort {
  return {
    path: () => location.pathname,
    push: (path) => history.pushState(null, "", path),
    replace: (path) => history.replaceState(null, "", path),
    onPop(listener) {
      window.addEventListener("popstate", listener);
      return () => window.removeEventListener("popstate", listener);
    },
  };
}
