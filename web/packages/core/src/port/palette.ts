import type { PaletteIndex, RecentItem } from "../model/palette";

// コマンドパレット(A5c)の口。実装は api パッケージにある。
export interface PalettePort {
  // index は、アイテム・プロジェクトの候補(パレットを開いたときに 1 回読む。A5c-2)。
  index(): Promise<PaletteIndex>;
  // recent は、最近開いたアイテム(新しい順に 8 件まで)。
  recent(): Promise<RecentItem[]>;
  // opened は、アイテムを開いたとこのマシンに覚える。
  opened(storeId: string, itemId: string): Promise<void>;
}
