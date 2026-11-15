import { describe, expect, it } from "vitest";
import { myTasksBadge, myTasksEmpty } from "./view";

describe("myTasks view", () => {
  it("empty texts", () => {
    expect(myTasksEmpty(0)).toEqual({
      title: "担当しているタスクはありません",
      body: "新しく割り当てられると、ここと受信箱に出ます。",
    });
    expect(myTasksEmpty(3)).toEqual({
      title: "一致するものはありません",
      body: "絞り込みを変えてみてください。",
    });
  });
  it("badge", () => {
    expect(myTasksBadge(2)).toBe("未完了 2 件");
  });
});
