export type Frame = { width: number; height: number };
export type Point = { x: number; y: number };
export type Bar = { x: number; y: number; width: number; height: number };

// 縦軸の切りのよい最大値: 最大値以上で最小の {1, 2, 5} × 10^k。1 未満は 1。
export function niceMax(max: number): number {
  if (max <= 1) return 1;
  const exp = 10 ** Math.floor(Math.log10(max));
  for (const m of [1, 2, 5, 10]) {
    if (m * exp >= max) return m * exp;
  }
  return 10 * exp;
}

// 目盛りは 0・最大値の半分・最大値の 3 本。
export function ticks(max: number): number[] {
  return [0, max / 2, max];
}

// max は 0 より大きい前提(呼び出し側は niceMax の値を渡す)。
const yOf = (value: number, max: number, f: Frame): number => f.height - (value / max) * f.height;

const xOf = (i: number, n: number, f: Frame): number => (n <= 1 ? 0 : (i / (n - 1)) * f.width);

// values は 1 日 1 値。x は null も含めた配列の位置で決め(1 要素なら 0)、null の点は飛ばす。
export function linePoints(values: (number | null)[], max: number, f: Frame): Point[] {
  const points: Point[] = [];
  values.forEach((v, i) => {
    if (v === null) return;
    points.push({ x: xOf(i, values.length, f), y: yOf(v, max, f) });
  });
  return points;
}

export function pathOf(points: Point[]): string {
  return points.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x} ${p.y}`).join(" ");
}

// layers[層][日]。下の層から、層ごとの面の d を返す。
export function stackPaths(layers: number[][], max: number, f: Frame): string[] {
  const days = layers[0]?.length ?? 0;
  let lower = new Array<number>(days).fill(0);
  return layers.map((layer) => {
    const upper = lower.map((base, d) => base + (layer[d] ?? 0));
    const top = upper.map((v, d) => ({ x: xOf(d, days, f), y: yOf(v, max, f) }));
    const bottom = lower.map((v, d) => ({ x: xOf(d, days, f), y: yOf(v, max, f) })).reverse();
    lower = upper;
    return `${pathOf([...top, ...bottom])} Z`;
  });
}

// 群の幅のうち棒が使う割合。残りは群の間の余白にする。
const BAR_RATIO = 0.7;

// 群ごとに棒を並べる。max は 0 より大きい前提。
export function bars(groups: number[][], max: number, f: Frame): Bar[][] {
  const slot = f.width / groups.length;
  return groups.map((group, g) => {
    const inner = slot * BAR_RATIO;
    const width = inner / group.length;
    const left = g * slot + (slot - inner) / 2;
    return group.map((v, i) => {
      const height = (v / max) * f.height;
      return { x: left + i * width, y: f.height - height, width, height };
    });
  });
}
