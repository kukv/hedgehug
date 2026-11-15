import { Refused } from "../../model/errors";
import type { SettingsPort, SyncTiming } from "../../port/settings";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";

export const refusal =
  "英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください";
export const line = "team-a での あなた が @taro から tarou に変わります。";
export const network = "通信に失敗しました";

// fakeSettings は、メモリの口。calls に呼ばれた操作を残す。lines は preview が返す行。
// ログイン名に空白があれば、preview と setLogin は Refused で断る(サーバーの検査の代わり)。
// hold を true にすると、preview と setTiming は、holdSet なら setLogin は、release を呼ぶまで返らない。fail に操作の名前を入れると、その操作は Error で失敗する。
export function fakeSettings(login = "taro", lines: string[] = []) {
  const calls: string[] = [];
  const saved = { login, lines };
  let timing: SyncTiming = {
    auto: true,
    intervalMinutes: 5,
    afterWriteSeconds: 30,
    githubMinutes: 5,
  };
  let release = () => {};
  const ctl = { hold: false, holdSet: false, fail: new Set<string>() };
  const broken = (name: string) => {
    if (ctl.fail.has(name)) throw new Error(network);
  };
  const check = (l: string) => {
    if (/\s/.test(l)) throw new Refused(refusal);
  };
  const port: SettingsPort = {
    async login() {
      broken("login");
      return saved.login;
    },
    async previewLogin(l) {
      calls.push(`preview ${l}`);
      if (ctl.hold) await new Promise<void>((r) => (release = r));
      broken("preview");
      check(l);
      return saved.lines;
    },
    async setLogin(l) {
      calls.push(`set ${l}`);
      if (ctl.holdSet) await new Promise<void>((r) => (release = r));
      broken("set");
      check(l);
      saved.login = l;
    },
    async timing() {
      broken("timing");
      return timing;
    },
    async setTiming(t) {
      calls.push(`timing ${JSON.stringify(t)}`);
      if (ctl.hold) await new Promise<void>((r) => (release = r));
      broken("timing");
      timing = t;
    },
  };
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  const errors = () =>
    get(toasts.list)
      .filter((t) => t.error)
      .map((t) => t.text);
  return { port, calls, saved, ctl, release: () => release(), toasts, errors };
}
