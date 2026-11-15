import type { SearchRow } from "../features/search/types";

// マイタスクの口(E4a)。実装は api パッケージにある。filter は絞り込みの文字列(基本のクエリに足す)。
export interface MyTasksPort {
  list(filter: string): Promise<{ errors: string[]; rows: SearchRow[] }>;
}
