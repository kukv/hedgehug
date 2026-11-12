// 表示中の Store。名前・色・閲覧のみはこのマシンの設定(01)。
export interface StoreInfo {
  id: string;
  name: string;
  // color は、ui のトークン --t-<色> の名前。
  color: string;
  readOnly: boolean;
}
