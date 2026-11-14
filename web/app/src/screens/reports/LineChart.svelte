<script lang="ts">
  import {
    amount,
    type BurnDay,
    formatDay,
    linePoints,
    niceMax,
    pathOf,
    type ReportUnit,
    ticks,
  } from "@hedgehug/core";

  // バーンダウンの折れ線。「理想」は点線、「残り」は実線で最後の点に値を添える。色だけに頼らず、凡例は文字で書く。
  interface Props {
    days: BurnDay[];
    unit: ReportUnit;
    thisYear: number;
  }

  let { days, unit, thisYear }: Props = $props();

  const W = 600;
  const H = 220;
  const pad = { left: 44, right: 64, top: 14, bottom: 28 };
  const frame = { width: W - pad.left - pad.right, height: H - pad.top - pad.bottom };

  const peak = $derived(Math.max(0, ...days.flatMap((d) => [d.total, d.ideal, d.remaining ?? 0])));
  const max = $derived(niceMax(peak));
  const yOf = (v: number) => linePoints([v], max, frame)[0]?.y ?? 0;
  const ideal = $derived(
    pathOf(
      linePoints(
        days.map((d) => d.ideal),
        max,
        frame,
      ),
    ),
  );
  const remaining = $derived(
    linePoints(
      days.map((d) => d.remaining),
      max,
      frame,
    ),
  );
  const lastValue = $derived(days.findLast((d) => d.remaining !== null)?.remaining ?? null);
  const last = $derived(remaining.at(-1));
</script>

<figure>
  <svg viewBox="0 0 {W} {H}" role="img" aria-label="バーンダウン">
    <g transform="translate({pad.left} {pad.top})">
      {#each ticks(max) as tick (tick)}
        <line class="grid" x1="0" x2={frame.width} y1={yOf(tick)} y2={yOf(tick)} />
        <text class="axis" x="-6" y={yOf(tick)} text-anchor="end" dominant-baseline="middle">
          {tick}
        </text>
      {/each}
      <path class="ideal" d={ideal} />
      <path class="remaining" d={pathOf(remaining)} />
      {#if last && lastValue !== null}
        <circle class="dot" cx={last.x} cy={last.y} r="3" />
        <text class="value" x={last.x + 6} y={last.y} dominant-baseline="middle">
          {amount(lastValue, unit)}
        </text>
      {/if}
      <text class="axis" x="0" y={frame.height + 18}>
        {formatDay(days[0]?.date ?? "", thisYear)}
      </text>
      <text class="axis" x={frame.width} y={frame.height + 18} text-anchor="end">
        {formatDay(days.at(-1)?.date ?? "", thisYear)}
      </text>
    </g>
  </svg>
  <figcaption>
    <span><i class="key ideal-key"></i>理想</span>
    <span><i class="key remaining-key"></i>残り</span>
  </figcaption>
</figure>

<style>
  figure {
    margin: 8px 0 0;
  }
  svg {
    width: 100%;
    max-width: 640px;
    height: auto;
  }
  .grid {
    stroke: var(--line);
  }
  .axis {
    font-size: 11px;
    fill: var(--muted);
  }
  .value {
    font-size: 12px;
    fill: var(--ink);
  }
  path {
    fill: none;
    stroke-width: 2;
  }
  .ideal {
    stroke: var(--muted-2);
    stroke-dasharray: 5 4;
  }
  .remaining {
    stroke: var(--accent);
  }
  .dot {
    fill: var(--accent);
  }
  figcaption {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: var(--muted);
  }
  .key {
    display: inline-block;
    width: 18px;
    margin-right: 6px;
    vertical-align: middle;
    border-top: 2px solid;
  }
  .ideal-key {
    border-top-style: dashed;
    border-color: var(--muted-2);
  }
  .remaining-key {
    border-color: var(--accent);
  }
</style>
