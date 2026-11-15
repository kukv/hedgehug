import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createNameDialog } from "./nameDialog";

// 返事を手で返せる submit。
function pending() {
  const calls: string[] = [];
  let reply: (m: string | null) => void = () => {};
  const submit = (name: string) => {
    calls.push(name);
    return new Promise<string | null>((r) => {
      reply = r;
    });
  };
  return { calls, submit, reply: (m: string | null) => reply(m) };
}

describe("createNameDialog (E3b-10)", () => {
  it("closes when the submit succeeds", async () => {
    const d = createNameDialog();
    d.open(async () => null);
    expect(get(d.state)).toEqual({ error: null });
    await d.save("バグ");
    expect(get(d.state)).toBeNull();
  });

  it("stays open with the refusal as error", async () => {
    const d = createNameDialog();
    d.open(async () => "名前を入力してください");
    await d.save("");
    expect(get(d.state)).toEqual({ error: "名前を入力してください" });
  });

  it("ignores a second save while waiting for the reply", async () => {
    const p = pending();
    const d = createNameDialog();
    d.open(p.submit);
    const first = d.save("a");
    void d.save("a");
    expect(p.calls).toEqual(["a"]);
    p.reply("断り");
    await first;
    expect(get(d.state)).toEqual({ error: "断り" });
  });

  it("does not reopen when a refusal arrives after closing", async () => {
    const p = pending();
    const d = createNameDialog();
    d.open(p.submit);
    const saving = d.save("a");
    d.close();
    p.reply("断り");
    await saving;
    expect(get(d.state)).toBeNull();
  });
});
