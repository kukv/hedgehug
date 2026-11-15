// 保存したビューの口(E3b)。実装は api パッケージにある。422 の断りは、断りの文を message に持つ Refused で投げる。

export interface ViewContentDto {
  query: string;
  layout: "tree" | "table";
  table: { columns: string[]; group: string; sort: { key: string; desc: boolean } | null };
}

export interface ViewDto {
  id: string;
  name: string;
  content: ViewContentDto;
}

export interface ViewsPort {
  list(): Promise<ViewDto[]>;
  create(name: string, content: ViewContentDto): Promise<ViewDto>;
  save(id: string, content: ViewContentDto): Promise<ViewDto>;
  rename(id: string, name: string): Promise<ViewDto>;
  duplicate(id: string): Promise<ViewDto>;
  remove(id: string): Promise<void>;
  // keyImpact は、そのワークスペースの追加プロパティを消すと、保存したビューの条件が使えなくなるか。
  keyImpact(storeId: string, workspaceId: string, key: string): Promise<boolean>;
}
