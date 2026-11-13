import type { Account, AccountRow, SignInCode, SignInResult } from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get } from "../../reactive/store";
import { createToasts, type Timer } from "../toasts/toasts";

export function row(id: string, patch: Partial<AccountRow> = {}): AccountRow {
  return {
    id,
    name: `${id} の名前`,
    login: id.toLowerCase(),
    method: "gh",
    status: "ok",
    reason: "",
    orgs: [],
    ...patch,
  };
}

export function code(id: string): SignInCode {
  return {
    id,
    userCode: `CODE-${id}`,
    verificationUri: "https://github.com/login/device",
    expiresInSec: 900,
    intervalSec: 5,
  };
}

export const waiting: SignInResult = {
  status: "waiting",
  message: "",
  intervalSec: 5,
  account: null,
};

// fakeAccounts は、メモリの口。calls に呼ばれた操作を「名前 引数…」(省いた引数は -)で残す。
// refuse[名前] があれば、その操作を Refused(その文言)で断る。impacts は削除・変更の確認に足す行。
// polls は pollSignIn が順に返す答え(Error なら投げる。尽きたら waiting)。startSignIn のコードは S1 から順に振る。
export function fakeAccounts(rows: AccountRow[] = []) {
  const calls: string[] = [];
  const refuse: Record<string, string> = {};
  const impacts = { delete: [] as string[], edit: [] as string[] };
  const polls: (SignInResult | Error)[] = [];
  // rechecks は、recheck が返す答え。
  const rechecks: { cleared: string[]; still: string[] }[] = [];
  let started = 0;
  function op(name: string, ...args: (string | undefined)[]) {
    calls.push([name, ...args.map((a) => a ?? "-")].join(" "));
    const m = refuse[name];
    if (m !== undefined) throw new Refused(m);
  }
  function find(id: string): AccountRow {
    const r = rows.find((x) => x.id === id);
    if (!r) throw new NotFound("not found");
    return r;
  }
  const toAccount = (r: AccountRow): Account => ({
    id: r.id,
    name: r.name,
    login: r.login,
    method: r.method,
  });
  const port: AccountsPort = {
    list: async () => rows.map((r) => ({ ...r })),
    async add(v) {
      op("add", v.name, v.login, v.method);
      const r = row(`A${rows.length + 1}`, { ...v });
      rows.push(r);
      return toAccount(r);
    },
    async edit(id, v) {
      op("edit", id, v.name, v.login, v.method);
      const r = find(id);
      Object.assign(r, v);
      return toAccount(r);
    },
    async reauth(id) {
      op("reauth", id);
      const r = find(id);
      r.status = "ok";
      r.reason = "";
      return toAccount(r);
    },
    async recheck(id) {
      op("recheck", id);
      find(id);
      return rechecks.shift() ?? { cleared: [], still: [] };
    },
    async remove(id) {
      op("remove", id);
      find(id);
      rows.splice(
        rows.findIndex((x) => x.id === id),
        1,
      );
    },
    async deleteImpact(id) {
      op("deleteImpact", id);
      return [`「${find(id).name}」を削除します。`, ...impacts.delete];
    },
    async editImpact(id, login) {
      op("editImpact", id, login);
      find(id);
      return impacts.edit;
    },
    async startSignIn(input) {
      op("startSignIn", input.account, input.name, input.login);
      started += 1;
      return code(`S${started}`);
    },
    async pollSignIn(id) {
      op("pollSignIn", id);
      const p = polls.shift() ?? waiting;
      if (p instanceof Error) throw p;
      return p;
    },
    async cancelSignIn(id) {
      op("cancelSignIn", id);
    },
  };
  return { port, rows, calls, refuse, impacts, polls, rechecks };
}

// fakeTimer は、時間を手で進める時計。
export function fakeTimer() {
  let now = 0;
  let seq = 0;
  const pending = new Map<number, { at: number; fn: () => void }>();
  const timer: Timer = {
    set(fn, ms) {
      seq += 1;
      pending.set(seq, { at: now + ms, fn });
      return seq;
    },
    clear(handle) {
      pending.delete(handle as number);
    },
  };
  const advance = (ms: number) => {
    now += ms;
    for (const [h, p] of [...pending]) {
      if (p.at <= now) {
        pending.delete(h);
        p.fn();
      }
    }
  };
  return { timer, advance, pending };
}

// fakeToasts は、出したトーストの文を読めるトースト。
export function fakeToasts() {
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  return { toasts, texts: () => get(toasts.list).map((t) => t.text) };
}

// core の lib は ES2023 だけで setTimeout の型がないため、ここで宣言する。
declare function setTimeout(fn: () => void, ms: number): unknown;

// flush は、待っている Promise の続きを進める。
export const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
