import type {
  Located,
  TransferDestination,
  TransferInput,
  TransferPreview,
  TransferResult,
  TransferTarget,
} from "../model/transfer";

// 別のワークスペース・別の Store への移動(A5a)の口。実装は api パッケージにある。
// 失敗は、断りなら Refused(message は画面にそのまま出す文)、ないものなら NotFound、それ以外はそのまま投げる。
export interface TransfersPort {
  // targets は、移す先の候補(絞り込みと並びはサーバーが決める。A5a-13)。
  targets(storeId: string, itemId: string): Promise<TransferTarget[]>;
  // preview は、移動先でのダイアログの予告(A5a-14)。
  preview(storeId: string, itemId: string, to: TransferDestination): Promise<TransferPreview>;
  transfer(storeId: string, itemId: string, input: TransferInput): Promise<TransferResult>;
  // locate は、アドレスの Store 名と ID(旧 ID・旧い接頭辞でもよい)をいまの場所にする。ないものは NotFound。
  locate(storeName: string, id: string): Promise<Located>;
}
