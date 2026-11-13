import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { Project } from "../../model/project";
import type { ProjectsPort } from "../../port/projects";
import { get } from "../../reactive/store";
import { project } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createProjectEditor } from "./editor";

// 偽の口: 呼び出しを覚え、断らせる・失敗させる・返事を止めることができる。reload は、保存のあとの読み直し
// (既定はすぐ終わる)。
function setup(reload: () => Promise<void> = async () => {}) {
  const calls: string[] = [];
  let failure: Error | null = null;
  let held: Promise<void> | null = null;
  const base: Project = { ...project("S1", "S1-W1", 1), repos: [] };
  const done = async (call: string) => {
    calls.push(call);
    const fail = failure;
    const wait = held;
    failure = null;
    held = null;
    if (wait !== null) await wait;
    if (fail !== null) throw fail;
    return base;
  };
  const projects: ProjectsPort = {
    get: async () => {
      throw new Error("unused");
    },
    create: async () => {
      throw new Error("unused");
    },
    rename: (s, p, title) => done(`rename ${s} ${p} ${title}`),
    setStatus: (s, p, status) => done(`setStatus ${s} ${p} ${status}`),
    setOwner: (s, p, owner) => done(`setOwner ${s} ${p} ${owner}`),
    setDate: (s, p, field, value) => done(`setDate ${s} ${p} ${field} ${value}`),
    addRepo: (s, p, repo) => done(`addRepo ${s} ${p} ${repo}`),
    removeRepo: (s, p, repo) => done(`removeRepo ${s} ${p} ${repo}`),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const editor = createProjectEditor({
    projects,
    toasts,
    onChanged: async () => {
      changed += 1;
      await reload();
    },
  });
  return {
    editor,
    base,
    calls,
    toasts,
    changed: () => changed,
    refuseNext: (message: string) => {
      failure = new Refused(message);
    },
    failNext: (message: string) => {
      failure = new Error(message);
    },
    // holdNext は、次の呼び出しの返事を、返した関数を呼ぶまで止める。
    holdNext: () => {
      let release = () => {};
      held = new Promise<void>((r) => {
        release = r;
      });
      return release;
    },
  };
}

// flush は、終わっている非同期の続きを流す。
async function flush() {
  for (let i = 0; i < 20; i++) await Promise.resolve();
}

describe("createProjectEditor", () => {
  it("renames with the trimmed title, then reloads", async () => {
    const { editor, base, calls, changed } = setup();
    expect(await editor.rename(base, "  決済  ")).toBe(true);
    expect(calls).toEqual(["rename S1 S1-P1 決済"]);
    expect(changed()).toBe(1);
  });

  it("sends nothing when the value is the same as the saved one", async () => {
    const { editor, base, calls, changed } = setup();
    expect(await editor.rename(base, ` ${base.title} `)).toBe(true);
    expect(await editor.setStatus(base, base.status)).toBe(true);
    expect(await editor.setOwner(base, base.owner)).toBe(true);
    expect(await editor.setDate(base, "start", base.start)).toBe(true);
    expect(calls).toEqual([]);
    expect(changed()).toBe(0);
  });

  // 保存のあとの読み直しが終わるまでは、画面のプロジェクトは保存の前の値のままなので、この画面で保存した値と比べる
  // (たとえば開始日を入れてすぐ消すと、読み直しの前の値と同じに見えて、消した値が保存されない)。
  it("compares with the value it saved while the reload after the save is running", async () => {
    let release = () => {};
    const gate = new Promise<void>((r) => {
      release = r;
    });
    const { editor, base, calls } = setup(() => gate);
    const saving = editor.setDate(base, "start", "2026-11-20");
    for (let i = 0; i < 20; i++) await Promise.resolve();
    // 保存した値のままの欄と、触っていない欄は送らない。
    expect(await editor.setDate(base, "start", "2026-11-20")).toBe(true);
    expect(await editor.rename(base, base.title)).toBe(true);
    // 保存した値から戻す変更は、画面のプロジェクトの値と同じでも送る。
    const clearing = editor.setDate(base, "start", base.start);
    for (let i = 0; i < 20; i++) await Promise.resolve();
    expect(calls).toEqual([
      "setDate S1 S1-P1 start 2026-11-20",
      `setDate S1 S1-P1 start ${base.start}`,
    ]);
    release();
    expect(await Promise.all([saving, clearing])).toEqual([true, true]);
    // 読み直しが終わったら、画面のプロジェクトの値と比べる。
    expect(await editor.setDate(base, "start", base.start)).toBe(true);
    expect(calls).toHaveLength(2);
  });

  // 保存の返事を待つあいだも、送った値と比べる(日付を入れて欄を離れ、返事の前に消して離れると、消した値が送られない)。
  it("compares with the value being sent while the save is in flight", async () => {
    const { editor, base, calls, holdNext } = setup();
    const release = holdNext();
    const saving = editor.setDate(base, "start", "2026-11-20");
    await flush();
    const clearing = editor.setDate(base, "start", base.start);
    await flush();
    expect(calls).toEqual(["setDate S1 S1-P1 start 2026-11-20", "setDate S1 S1-P1 start "]);
    release();
    expect(await Promise.all([saving, clearing])).toEqual([true, true]);
  });

  // 断られた・失敗した値は、送らなかったことに戻す(ほかの保存が途中でも、画面のプロジェクトの値と比べる)。
  it("rolls the value back when the save is refused or fails", async () => {
    const { editor, base, calls, holdNext, refuseNext, failNext } = setup();
    const release = holdNext();
    const renaming = editor.rename(base, "決済");
    await flush();
    refuseNext("終了日は開始日以降にしてください");
    expect(await editor.setDate(base, "end", "2026-11-01")).toBe(false);
    failNext("network");
    await expect(editor.setDate(base, "start", "2026-11-20")).rejects.toThrow("network");
    expect(await editor.setDate(base, "end", base.end)).toBe(true);
    expect(await editor.setDate(base, "start", base.start)).toBe(true);
    expect(calls).toEqual([
      "rename S1 S1-P1 決済",
      "setDate S1 S1-P1 end 2026-11-01",
      "setDate S1 S1-P1 start 2026-11-20",
    ]);
    release();
    expect(await renaming).toBe(true);
  });

  it("shows the server's refusal as an error toast and returns false", async () => {
    const { editor, base, toasts, changed, refuseNext } = setup();
    refuseNext("タイトルは空にできません");
    expect(await editor.rename(base, "   ")).toBe(false);
    const first = get(toasts.list)[0];
    expect(first?.text).toBe("タイトルは空にできません");
    expect(first?.error).toBe(true);
    expect(changed()).toBe(0);
  });

  it("sends the status, the owner and each date", async () => {
    const { editor, base, calls, changed } = setup();
    expect(await editor.setStatus(base, "InProgress")).toBe(true);
    expect(await editor.setOwner(base, "")).toBe(true);
    expect(await editor.setDate(base, "end", "2026-11-30")).toBe(true);
    expect(calls).toEqual([
      "setStatus S1 S1-P1 InProgress",
      "setOwner S1 S1-P1 ",
      "setDate S1 S1-P1 end 2026-11-30",
    ]);
    expect(changed()).toBe(3);
  });

  it("refuses a date range through the toast and leaves the form to be reset", async () => {
    const { editor, base, toasts, refuseNext } = setup();
    refuseNext("終了日は開始日以降にしてください");
    expect(await editor.setDate({ ...base, start: "2026-11-02" }, "end", "2026-11-01")).toBe(false);
    expect(get(toasts.list)[0]?.text).toBe("終了日は開始日以降にしてください");
  });

  it("adds a repo with the typed text and returns an empty message", async () => {
    const { editor, base, calls, changed, toasts } = setup();
    expect(await editor.addRepo(base, " example-org/web ")).toBe("");
    expect(calls).toEqual(["addRepo S1 S1-P1  example-org/web "]);
    expect(changed()).toBe(1);
    expect(get(toasts.list)).toEqual([]); // 追加ではトーストを出さない
  });

  it("returns the refusal of an add as the message, without a toast or a reload", async () => {
    const { editor, base, changed, toasts, refuseNext } = setup();
    refuseNext("もう追加されています");
    expect(await editor.addRepo(base, "example-org/web")).toBe("もう追加されています");
    expect(get(toasts.list)).toEqual([]);
    expect(changed()).toBe(0);
  });

  it("removes a repo, reloads and tells it with a toast", async () => {
    const { editor, base, calls, changed, toasts } = setup();
    await editor.removeRepo(base, "example-org/web");
    expect(calls).toEqual(["removeRepo S1 S1-P1 example-org/web"]);
    expect(changed()).toBe(1);
    const first = get(toasts.list)[0];
    expect(first?.text).toBe("example-org/web を削除しました");
    expect(first?.error).toBe(false);
  });
});
