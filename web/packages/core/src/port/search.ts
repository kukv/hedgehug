import type { SearchResult, SearchTableRequest } from "../features/search/types";

// 検索の口。実装は api パッケージにある。
export interface SearchPort {
  search(query: string, table?: SearchTableRequest): Promise<SearchResult>;
}

// 画面の状態の口(E1-16)。body は画面ごとの JSON の文字列で、サーバーは中身を見ない。
export interface ScreenStatesPort {
  // get は保存した文字列。まだなければ null。
  get(screen: string): Promise<string | null>;
  put(screen: string, body: string): Promise<void>;
}
