// コマンドパレット(02「コマンドパレット」、トラック A5c)の、サーバーが返す候補。

// PaletteAlias は、旧 ID。at は移動した日時で、旧い接頭辞から作ったものは null(A5c-4)。
export interface PaletteAlias {
  id: string;
  at: string | null;
}
// PaletteItem は、表示中の Store の削除されていないアイテム 1 件。display はいまの ID(HH-12)。
export interface PaletteItem {
  storeId: string;
  workspaceId: string;
  projectId: string;
  itemId: string;
  number: number;
  display: string;
  title: string;
  projectTitle: string;
  aliases: PaletteAlias[];
}
// PaletteProject は、表示中の Store の削除されていないプロジェクト 1 件。display はいまの ID(HH-P1)。
export interface PaletteProject {
  storeId: string;
  workspaceId: string;
  projectId: string;
  display: string;
  title: string;
  workspaceName: string;
  aliases: PaletteAlias[];
}
// PaletteIndex は、パレットのアイテム・プロジェクトの候補(並びは問わない。A5c-2・A5c-3)。
export interface PaletteIndex {
  items: PaletteItem[];
  projects: PaletteProject[];
}
// RecentItem は、最近開いたアイテム 1 件(新しい順。A5c-13)。storeId・display は、いま見つかった Store とそこでの ID。
export interface RecentItem {
  storeId: string;
  itemId: string;
  display: string;
  title: string;
  projectTitle: string;
}
