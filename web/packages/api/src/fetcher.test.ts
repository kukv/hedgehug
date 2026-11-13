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
  onRestored(() => {});
});

describe("fetcher の Hedgehug-Restored", () => {
  it("成功した応答のヘッダーを ID の並びにして受け手に渡す", async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200, { "Hedgehug-Restored": "HH-1" });
    await fetcher("/x");
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith(["HH-1"]);
  });

  it('複数あるときは ", " で分ける', async () => {
    const listener = vi.fn();
    onRestored(listener);
    respond(200, { "Hedgehug-Restored": "HH-1, HH-2" });
    await fetcher("/x");
    expect(listener).toHaveBeenCalledWith(["HH-1", "HH-2"]);
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
