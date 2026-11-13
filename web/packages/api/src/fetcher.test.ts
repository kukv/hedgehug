import { afterEach, describe, expect, it, vi } from "vitest";
import { fetcher, onRestored } from "./fetcher";

function respond(status: number, headers: Record<string, string> = {}): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response("{}", { status, headers })),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  onRestored(null);
});

describe("fetcher の Hedgehug-Restored", () => {
  it("成功した応答のヘッダーを ID の並びにして受け手に渡す", async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200, { "Hedgehug-Restored": "HH-1" });
    await fetcher("/x");
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith([{ kind: "id", text: "HH-1" }]);
  });

  it('複数あるときは ", " で分ける', async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200, { "Hedgehug-Restored": "HH-1, HH-2" });
    await fetcher("/x");
    expect(listener).toHaveBeenCalledWith([
      { kind: "id", text: "HH-1" },
      { kind: "id", text: "HH-2" },
    ]);
  });

  it("ID・名前・コメントのヘッダーをこの順に種類を付けて 1 回で渡す", async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200, {
      "Hedgehug-Restored-Comment": "HH-2",
      "Hedgehug-Restored-Name": encodeURIComponent("レビュー, 待ち 「A」"),
      "Hedgehug-Restored": "HH-1",
    });
    await fetcher("/x");
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith([
      { kind: "id", text: "HH-1" },
      { kind: "name", text: "レビュー, 待ち 「A」" },
      { kind: "comment", text: "HH-2" },
    ]);
  });

  it('名前のヘッダーが 2 つ(fetch が ", " でつなぐ)でも 2 つに分かれる', async () => {
    const listener = vi.fn();
    onRestored(listener);
    const headers = new Headers();
    headers.append("Hedgehug-Restored-Name", "A");
    headers.append("Hedgehug-Restored-Name", encodeURIComponent("B C"));
    respond(200, Object.fromEntries(headers));
    await fetcher("/x");
    expect(listener).toHaveBeenCalledWith([
      { kind: "name", text: "A" },
      { kind: "name", text: "B C" },
    ]);
  });

  it("ヘッダーがない応答では呼ばない", async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200);
    await fetcher("/x");
    expect(listener).not.toHaveBeenCalled();
  });

  it("空のヘッダーでは呼ばない", async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200, { "Hedgehug-Restored": " , " });
    await fetcher("/x");
    expect(listener).not.toHaveBeenCalled();
  });

  it("失敗した応答では呼ばない", async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(422, { "Hedgehug-Restored": "HH-1" });
    await expect(fetcher("/x")).rejects.toThrow();
    expect(listener).not.toHaveBeenCalled();
  });

  it("受け手がなくても成功する", async () => {
    respond(200, { "Hedgehug-Restored": "HH-1" });
    await expect(fetcher("/x")).resolves.toEqual({});
  });
});
