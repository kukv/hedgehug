import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { SettingsPort, SyncTiming } from "../../port/settings";
import { get } from "../../reactive/store";
import {
  afterWriteChoices,
  createLoginField,
  createSyncTimingField,
  githubChoices,
  intervalChoices,
  keyboardRows,
  minutesLabel,
  secondsLabel,
  treeKeyRows,
} from "./general";

const refusal = "英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください";
const line = "team-a での あなた が @taro から tarou に変わります。";

// fakeSettings は、メモリの口。calls に呼ばれた操作を残す。lines は preview が返す行。
// ログイン名に空白があれば、preview と setLogin は Refused で断る(サーバーの検査の代わり)。
// hold を true にすると、preview は release を呼ぶまで返らない。
function fakeSettings(login = "taro", lines: string[] = []) {
  const calls: string[] = [];
  const saved = { login, lines };
  let timing: SyncTiming = {
    auto: true,
    intervalMinutes: 5,
    afterWriteSeconds: 30,
    githubMinutes: 5,
  };
  let release = () => {};
  const ctl = { hold: false };
  const check = (l: string) => {
    if (/\s/.test(l)) throw new Refused(refusal);
  };
  const port: SettingsPort = {
    login: async () => saved.login,
    async previewLogin(l) {
      calls.push(`preview ${l}`);
      if (ctl.hold) await new Promise<void>((r) => (release = r));
      check(l);
      return saved.lines;
    },
    async setLogin(l) {
      calls.push(`set ${l}`);
      check(l);
      saved.login = l;
    },
    timing: async () => timing,
    async setTiming(t) {
      calls.push(`timing ${JSON.stringify(t)}`);
      timing = t;
    },
  };
  return { port, calls, saved, ctl, release: () => release() };
}

async function loaded(login = "taro", lines: string[] = []) {
  const f = fakeSettings(login, lines);
  const field = createLoginField(f.port);
  await field.load();
  return { ...f, field };
}

describe("choices", () => {
  it("lists the choices of 08 with their labels", () => {
    expect(intervalChoices.map(minutesLabel)).toEqual([
      "1 分ごと",
      "5 分ごと",
      "15 分ごと",
      "30 分ごと",
      "60 分ごと",
    ]);
    expect(afterWriteChoices.map(secondsLabel)).toEqual(["10 秒", "30 秒", "60 秒"]);
    expect(githubChoices.map(minutesLabel)).toEqual([
      "5 分ごと",
      "15 分ごと",
      "30 分ごと",
      "60 分ごと",
    ]);
  });
});

describe("keyboard tables", () => {
  it("lists the 5 keys of 01 in order, and the tree keys of 04", () => {
    expect(keyboardRows.map((r) => r.key)).toEqual([
      "Ctrl+K(Cmd+K)",
      "C",
      "/",
      "Esc",
      "Shift+Ctrl+S(Shift+Cmd+S)",
    ]);
    expect(treeKeyRows.map((r) => r.key)).toEqual([
      "↑ ↓",
      "→",
      "←",
      "Enter",
      "Ctrl+Enter(Cmd+Enter)",
      "Space",
      "S",
      "K",
      "Alt+↑、Alt+↓",
      "Alt+Shift+↑、Alt+Shift+↓",
      "C",
      "メニューキー、Shift+F10",
    ]);
  });
});

describe("createLoginField", () => {
  it("loads the saved login into the field", async () => {
    const { field } = await loaded();
    expect(get(field.state)).toEqual({ text: "taro", error: "", confirm: null });
  });

  it("does nothing when the trimmed text is the same", async () => {
    const { field, calls } = await loaded();
    field.edit(" taro ");
    await field.commit(" taro ");
    expect(calls).toEqual([]);
    expect(get(field.state).text).toBe("taro");
  });

  it("saves at once when no store changes its あなた", async () => {
    const { field, calls, saved } = await loaded();
    field.edit(" tarou ");
    await field.commit(" tarou ");
    expect(calls).toEqual(["preview tarou", "set tarou"]);
    expect(saved.login).toBe("tarou");
    expect(get(field.state)).toEqual({ text: "tarou", error: "", confirm: null });
  });

  it("commits once for Enter and the blur that follows it", async () => {
    const { field, calls, ctl, release } = await loaded();
    ctl.hold = true;
    field.edit("tarou");
    const enter = field.commit("tarou");
    const blur = field.commit("tarou");
    release();
    await Promise.all([enter, blur]);
    expect(calls).toEqual(["preview tarou", "set tarou"]);
  });

  it("asks before saving when a store changes its あなた, and ignores the blur meanwhile", async () => {
    const { field, calls, saved } = await loaded("taro", [line]);
    field.edit("tarou");
    await field.commit("tarou");
    expect(get(field.state).confirm).toEqual([line]);
    await field.commit("tarou");
    expect(calls).toEqual(["preview tarou"]);
    await field.confirm();
    expect(calls).toEqual(["preview tarou", "set tarou"]);
    expect(saved.login).toBe("tarou");
    expect(get(field.state)).toEqual({ text: "tarou", error: "", confirm: null });
  });

  it("goes back to the saved login on cancel", async () => {
    const { field, calls } = await loaded("taro", [line]);
    field.edit("tarou");
    await field.commit("tarou");
    field.cancel();
    expect(get(field.state)).toEqual({ text: "taro", error: "", confirm: null });
    expect(calls).toEqual(["preview tarou"]);
    // キャンセルのあとは、また確定できる。
    await field.commit("tarou");
    expect(calls).toEqual(["preview tarou", "preview tarou"]);
  });

  it("does nothing on cancel while the preview is running", async () => {
    const { field, calls, ctl, release } = await loaded();
    ctl.hold = true;
    const enter = field.commit("tarou");
    field.cancel();
    const blur = field.commit("tarou");
    release();
    await Promise.all([enter, blur]);
    expect(calls).toEqual(["preview tarou", "set tarou"]);
  });

  it("shows the refusal and goes back to the saved login, and the blur keeps the refusal", async () => {
    const { field, calls } = await loaded();
    field.edit("ta ro");
    await field.commit("ta ro");
    expect(get(field.state)).toEqual({ text: "taro", error: refusal, confirm: null });
    await field.commit("taro");
    expect(get(field.state).error).toBe(refusal);
    expect(calls).toEqual(["preview ta ro"]);
  });

  it("clears the refusal on the next commit that saves", async () => {
    const { field } = await loaded();
    await field.commit("ta ro");
    await field.commit("tarou");
    expect(get(field.state)).toEqual({ text: "tarou", error: "", confirm: null });
  });

  it("keeps what is being typed when the saved login is read again", async () => {
    const { field, saved } = await loaded();
    field.edit("tar");
    saved.login = "jiro";
    await field.load();
    expect(get(field.state).text).toBe("tar");
    await field.commit("jiro");
    expect(get(field.state).text).toBe("jiro");
  });

  it("takes a login changed elsewhere when nothing is being typed", async () => {
    const { field, saved } = await loaded();
    saved.login = "jiro";
    await field.load();
    expect(get(field.state).text).toBe("jiro");
  });
});

describe("createSyncTimingField", () => {
  it("loads the timing and saves a change", async () => {
    const f = fakeSettings();
    const timing = createSyncTimingField(f.port);
    expect(get(timing.state)).toBeNull();
    await timing.load();
    expect(get(timing.state)).toEqual({
      auto: true,
      intervalMinutes: 5,
      afterWriteSeconds: 30,
      githubMinutes: 5,
    });
    await timing.set({ intervalMinutes: 15 });
    expect(f.calls).toEqual([
      'timing {"auto":true,"intervalMinutes":15,"afterWriteSeconds":30,"githubMinutes":5}',
    ]);
    expect(get(timing.state)?.intervalMinutes).toBe(15);
  });
});
