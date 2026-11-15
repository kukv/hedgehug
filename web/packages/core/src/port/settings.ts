// SyncTiming は、設定「一般」の「同期のタイミング」(08)。このマシンだけの設定。
export interface SyncTiming {
  auto: boolean;
  intervalMinutes: number;
  afterWriteSeconds: number;
  githubMinutes: number;
}

// マシンの設定の口(ログイン名と同期のタイミング)。実装は api パッケージにある。断りは Refused で投げる。
export interface SettingsPort {
  login(): Promise<string>;
  // previewLogin は、ログイン名を変えたときの確認の本文の行(空なら確認を出さない)。
  previewLogin(login: string): Promise<string[]>;
  setLogin(login: string): Promise<void>;
  timing(): Promise<SyncTiming>;
  setTiming(timing: SyncTiming): Promise<void>;
}
