import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { fakeSettings, line, network, refusal } from "./fixtures.test-util";
import {
  afterWriteChoices,
  createLoginField,
  githubChoices,
  intervalChoices,
  keyboardRows,
  minutesLabel,
  secondsLabel,
  treeKeyRows,
} from "./general";

async function loaded(login = "taro", lines: string[] = []) {
  const f = fakeSettings(login, lines);
  const field = createLoginField(f.port, f.toasts);
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

  it("does not open the confirmation when cancel comes while the preview is running", async () => {
    const { field, calls, ctl, release } = await loaded("taro", [line]);
    ctl.hold = true;
    field.edit("tarou");
    const enter = field.commit("tarou");
    field.cancel();
    expect(get(field.state).text).toBe("taro");
    // preview が返るまでは、次の確定を受けない。
    const blur = field.commit("tarou");
    release();
    await Promise.all([enter, blur]);
    expect(get(field.state)).toEqual({ text: "taro", error: "", confirm: null });
    expect(calls).toEqual(["preview tarou"]);
    // 返ったあとは、また確定できる。
    ctl.hold = false;
    await field.commit("tarou");
    expect(get(field.state).confirm).toEqual([line]);
  });

  it("does not save when cancel comes while a preview without lines is running", async () => {
    const { field, calls, ctl, release } = await loaded();
    ctl.hold = true;
    const enter = field.commit("tarou");
    field.cancel();
    release();
    await enter;
    expect(calls).toEqual(["preview tarou"]);
    expect(get(field.state).text).toBe("taro");
  });

  it("clears the refusal when a later save fails for another reason", async () => {
    const { field, ctl, errors } = await loaded();
    await field.commit("ta ro");
    expect(get(field.state).error).toBe(refusal);
    ctl.fail.add("preview");
    await field.commit("tarou");
    expect(get(field.state)).toEqual({ text: "taro", error: "", confirm: null });
    expect(errors()).toEqual([network]);
  });

  it("closes the confirmation and goes back when saving fails, with an error toast", async () => {
    const { field, ctl, errors } = await loaded("taro", [line]);
    field.edit("tarou");
    await field.commit("tarou");
    ctl.fail.add("set");
    await field.confirm();
    expect(get(field.state)).toEqual({ text: "taro", error: "", confirm: null });
    expect(errors()).toEqual([network]);
    // 閉じたあとは、また確定できる。
    ctl.fail.clear();
    await field.commit("tarou");
    expect(get(field.state).confirm).toEqual([line]);
  });

  it("goes back when the preview fails, with an error toast", async () => {
    const { field, ctl, calls, errors } = await loaded();
    ctl.fail.add("preview");
    field.edit("tarou");
    await field.commit("tarou");
    expect(get(field.state)).toEqual({ text: "taro", error: "", confirm: null });
    expect(errors()).toEqual([network]);
    ctl.fail.clear();
    await field.commit("tarou");
    expect(calls).toEqual(["preview tarou", "preview tarou", "set tarou"]);
  });

  it("closes the confirmation on cancel while 変更 is saving", async () => {
    const { field, ctl, calls, release } = await loaded("taro", [line]);
    await field.commit("tarou");
    ctl.holdSet = true;
    const saving = field.confirm();
    field.cancel();
    expect(get(field.state).confirm).toBeNull();
    // 保存が終わるまでは、次の確定を受けない。
    await field.commit("jiro");
    release();
    await saving;
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

  it("shows an error toast when the login cannot be read", async () => {
    const { field, ctl, errors } = await loaded();
    ctl.fail.add("login");
    await field.load();
    expect(get(field.state).text).toBe("taro");
    expect(errors()).toEqual([network]);
  });

  it("takes a login changed elsewhere when nothing is being typed", async () => {
    const { field, saved } = await loaded();
    saved.login = "jiro";
    await field.load();
    expect(get(field.state).text).toBe("jiro");
  });
});
