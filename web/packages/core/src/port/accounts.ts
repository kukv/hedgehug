import type {
  Account,
  AccountRow,
  SignInCode,
  SignInResult,
  SignInStart,
  TokenValues,
} from "../model/account";

// アカウントの口。実装は api パッケージにある。断りは Refused、ないものは NotFound で投げる。
export interface AccountsPort {
  list(): Promise<AccountRow[]>;
  add(values: TokenValues): Promise<Account>;
  edit(id: string, values: TokenValues): Promise<Account>;
  reauth(id: string): Promise<Account>;
  remove(id: string): Promise<void>;
  // deleteImpact は「アカウントを削除」の本文の行、editImpact は「アカウントを変更」の本文の行(空なら確認を出さない)。
  deleteImpact(id: string): Promise<string[]>;
  editImpact(id: string, login: string): Promise<string[]>;
  startSignIn(input: SignInStart): Promise<SignInCode>;
  pollSignIn(id: string): Promise<SignInResult>;
  cancelSignIn(id: string): Promise<void>;
}
