import { type Readable, readonly, writable } from "../../reactive/store";

// 画面の全体の枠の状態。狭い幅でサイドバーを引き出しとして開け閉めする。
export interface Shell {
  drawerOpen: Readable<boolean>;
  openDrawer(): void;
  closeDrawer(): void;
  toggleDrawer(): void;
}

export function createShell(): Shell {
  const drawerOpen = writable(false);
  return {
    drawerOpen: readonly(drawerOpen),
    openDrawer: () => drawerOpen.set(true),
    closeDrawer: () => drawerOpen.set(false),
    toggleDrawer: () => drawerOpen.update((open) => !open),
  };
}
