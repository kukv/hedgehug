// 画面で使う型。API の型はそのまま使わず、api パッケージがこの型に直す(設計書 6 節)。
export interface SystemInfo {
  version: string;
}
