import type { ActivityFilter, ActivityPage } from "../model/activity";

// アクティビティの口。実装は api パッケージにある。
export interface ActivityPort {
  // list は、行を新しい順に 300 件まで読む。before は、前のページの最後の行の cursor。
  list(filter: ActivityFilter, before?: string): Promise<ActivityPage>;
}
