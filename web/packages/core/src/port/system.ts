import type { SystemInfo } from "../model/system";

// サーバーとの口。実装は api パッケージにある。
export interface SystemPort {
  getSystem(): Promise<SystemInfo>;
}
