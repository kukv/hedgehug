import { describe, expect, it } from "vitest";
import { type Bar, bars, linePoints, niceMax, pathOf, stackPaths, ticks } from "./chart";

const frame = { width: 200, height: 100 };

describe("niceMax", () => {
  it.each([
    [0, 1],
    [0.4, 1],
    [1, 1],
    [3, 5],
    [7, 10],
    [12, 20],
    [45, 50],
    [50, 50],
    [51, 100],
  ])("%s は %s", (max, want) => {
    expect(niceMax(max)).toBe(want);
  });
});

describe("ticks", () => {
  it("0・半分・最大の 3 本", () => {
    expect(ticks(10)).toEqual([0, 5, 10]);
  });
});

describe("linePoints", () => {
  it("x は位置、y は値の高さで決める", () => {
    expect(linePoints([8, 4, 0], 8, frame)).toEqual([
      { x: 0, y: 0 },
      { x: 100, y: 50 },
      { x: 200, y: 100 },
    ]);
  });

  it("null の点は飛ばし、点が 1 つなら x は 0", () => {
    expect(linePoints([8, null], 8, frame)).toEqual([{ x: 0, y: 0 }]);
    expect(linePoints([8], 8, frame)).toEqual([{ x: 0, y: 0 }]);
  });

  it("x は null を含めた配列の位置で決める", () => {
    expect(linePoints([null, 8], 8, frame)).toEqual([{ x: 200, y: 0 }]);
  });
});

describe("pathOf", () => {
  it("M と L でつなぐ", () => {
    expect(
      pathOf([
        { x: 0, y: 1 },
        { x: 2, y: 3 },
      ]),
    ).toBe("M 0 1 L 2 3");
  });

  it("点がなければ空文字", () => {
    expect(pathOf([])).toBe("");
  });
});

describe("stackPaths", () => {
  it("層ごとの面を下の層から返し、上端が積み上がる", () => {
    const paths = stackPaths(
      [
        [1, 1],
        [2, 0],
      ],
      4,
      { width: 100, height: 100 },
    );
    expect(paths).toHaveLength(2);
    // 面は「上端を左から右」「下端を右から左」の順に点を並べ、Z で閉じる。
    const points = (d: string) => d.match(/-?\d+(\.\d+)? -?\d+(\.\d+)?/g) ?? [];
    expect(points(paths[0] ?? "")).toEqual(["0 75", "100 75", "100 100", "0 100"]);
    // 上の層の下端は、下の層の上端(y=75)をなぞる。
    expect(points(paths[1] ?? "")).toEqual(["0 25", "100 75", "100 75", "0 75"]);
    for (const d of paths) expect(d.endsWith(" Z")).toBe(true);
  });
});

describe("bars", () => {
  it("高さは値に比例し、0 は 0", () => {
    const got = bars(
      [
        [10, 5],
        [0, 0],
      ],
      10,
      frame,
    );
    expect(got).toHaveLength(2);
    expect(got[0]?.[0]?.height).toBe(100);
    expect(got[0]?.[1]?.height).toBe(50);
    expect(got[1]?.[0]?.height).toBe(0);
    expect(got[0]?.[0]?.y).toBe(0);
    expect(got[0]?.[1]?.y).toBe(50);
  });

  it("群は横に並び、群の中の棒は重ならない", () => {
    const got = bars(
      [
        [1, 1],
        [1, 1],
      ],
      1,
      frame,
    );
    expect(got).toHaveLength(2);
    expect(got[0]).toHaveLength(2);
    expect(got[1]).toHaveLength(2);
    const [a, b] = got[0] as [Bar, Bar];
    const [next] = got[1] as [Bar, Bar];
    expect(a.x + a.width).toBeLessThanOrEqual(b.x);
    expect(b.x + b.width).toBeLessThanOrEqual(next.x);
  });
});
