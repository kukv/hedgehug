// ブラウザの履歴の口。実装は app にある(History API)。core は DOM を知らない。
export interface HistoryPort {
  // path は、いまのアドレスのパス(例 "/s/work/p/HH-P1/tree")。
  path(): string;
  push(path: string): void;
  replace(path: string): void;
  // onPop は、戻る・進むで path が変わったときに listener を呼ぶ。止める関数を返す。
  onPop(listener: () => void): () => void;
}
