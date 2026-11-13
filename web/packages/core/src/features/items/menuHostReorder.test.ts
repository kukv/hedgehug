import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { flush, gate, item } from "./fixtures.test-util";
import { reorderFailedMessage } from "./menuHost";
import { ctx, ro, setup, t3, t5, t7 } from "./menuHost.test-util";

describe("createItemMenuHost reorder", () => {
  it("moves up before the visible previous sibling, skipping rows in between", async () => {
    const { host, log, messages } = await setup();
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(log).toEqual([`reorder ${t7.id} before ${t3.id}`, "change"]);
    expect(get(host.state)).toBeNull();
    expect(messages()).toEqual([]);
  });

  it("moves down after the visible next sibling", async () => {
    const { host, log } = await setup();
    host.open(t5, ctx, 0, 0);
    await host.select("move-down");
    expect(log).toEqual([`reorder ${t5.id} after ${t3.id}`, "change"]);
  });

  it("reorders without the menu, saying whether it moved (the Alt+arrow keys)", async () => {
    const { host, log, fail } = await setup();
    expect(await host.reorder(t3, ctx.rows, "up")).toBe(true);
    expect(await host.reorder(t3, ctx.rows, "down")).toBe(true);
    expect(log).toEqual([
      `reorder ${t3.id} before ${t5.id}`,
      "change",
      `reorder ${t3.id} after ${t7.id}`,
      "change",
    ]);
    // 見えている兄弟の端と閲覧のみの Store は、何も呼ばずに false。断りも false で、読み直さない。
    expect(await host.reorder(t5, ctx.rows, "up")).toBe(false);
    expect(await host.reorder(ro, [ro, item("S2", "S2-P1", 2, "Task", null)], "down")).toBe(false);
    expect(log).toHaveLength(4);
    fail(new Refused("動かせません"));
    expect(await host.reorder(t3, ctx.rows, "up")).toBe(false);
    expect(log).toHaveLength(5);
    expect(log.at(-1)).toBe(`reorder ${t3.id} before ${t5.id}`);
  });

  it("does not start a second reorder while one is running", async () => {
    const { host, log, hold } = await setup();
    const g = gate();
    hold(g.wait);
    const first = host.reorder(t3, ctx.rows, "up");
    expect(await host.reorder(t7, ctx.rows, "up")).toBe(false);
    g.release();
    expect(await first).toBe(true);
    expect(log).toEqual([`reorder ${t3.id} before ${t5.id}`, "change"]);
  });

  it("does not start a second reorder until the reload after the first has finished", async () => {
    const { host, log, holdRefresh } = await setup();
    const g = gate();
    holdRefresh(g.wait);
    const first = host.reorder(t3, ctx.rows, "up");
    await flush();
    expect(log).toEqual([`reorder ${t3.id} before ${t5.id}`, "change"]);
    expect(await host.reorder(t7, ctx.rows, "up")).toBe(false);
    g.release();
    expect(await first).toBe(true);
    expect(log).toHaveLength(2);
  });

  it("keeps a menu opened while a reorder is running", async () => {
    const { host, hold } = await setup();
    const g = gate();
    hold(g.wait);
    host.open(t7, ctx, 0, 0);
    const running = host.select("move-up");
    host.open(t5, ctx, 3, 4);
    g.release();
    await running;
    expect(get(host.state)).toMatchObject({ item: t5, x: 3, y: 4 });
  });

  it("shows a refusal of the reorder as an error toast and does not reload", async () => {
    const { host, log, messages, fail } = await setup();
    fail(new Refused("動かせません"));
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(messages()).toEqual(["!動かせません"]);
    expect(log).toEqual([`reorder ${t7.id} before ${t3.id}`]);
  });

  it("shows another failure of the reorder with its own words, and reloads when the item is gone", async () => {
    const { host, log, messages, fail } = await setup();
    fail(new Error("network"));
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(messages()).toEqual([`!${reorderFailedMessage}`]);
    expect(reorderFailedMessage).toBe("順番を変更できませんでした");
    fail(new NotFound("gone"));
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(messages()).toHaveLength(1);
    expect(log.at(-1)).toBe("change");
  });
});
