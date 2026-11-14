import type { Report, ReportActive, ReportSelection } from "../model/report";

// レポートの口。実装は api パッケージにある。
export interface ReportsPort {
  // period は累積フロー図の期間。null なら API が直近 21 日にする。
  report(
    storeId: string,
    workspaceId: string,
    period: { from: string; to: string } | null,
  ): Promise<Report>;
  active(storeId: string, workspaceId: string): Promise<ReportActive>;
  // 送った項目だけを保存する。
  select(storeId: string, workspaceId: string, patch: Partial<ReportSelection>): Promise<void>;
}
