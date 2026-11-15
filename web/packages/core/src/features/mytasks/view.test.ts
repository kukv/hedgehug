import { describe, expect, it } from "vitest";
import { myTasksBadge, myTasksEmpty } from "./view";

describe("myTasks view", () => {
  it("empty texts", () => {
    const none = {
      title: "担当しているタスクはありません",
      body: "新しく割り当てられると、ここと受信箱に出ます。",
    };
    const unmatched = { title: "一致するものはありません", body: "絞り込みを変えてみてください。" };
    expect(myTasksEmpty(0, false)).toEqual(none);
    // 絞り込みに誤りがあるときは、絞り込む前の件数が 0 でも、絞り込みの文にする(E4a-8)。
    expect(myTasksEmpty(0, true)).toEqual(unmatched);
    expect(myTasksEmpty(3, false)).toEqual(unmatched);
    expect(myTasksEmpty(3, true)).toEqual(unmatched);
  });
  it("badge", () => {
    expect(myTasksBadge(2)).toBe("未完了 2 件");
  });
});
