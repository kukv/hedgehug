<script lang="ts">
  import {
    type FlowDay,
    type FlowLayer,
    formatDay,
    layerFill,
    linePoints,
    niceMax,
    stackPaths,
    ticks,
  } from "@hedgehug/core";

  // 累積フローの積み上げ面。層は下から積む。色だけに頼らず、凡例に層の名前を文字で書く。
  interface Props {
    layers: FlowLayer[];
    flow: FlowDay[];
    thisYear: number;
  }

  let { layers, flow, thisYear }: Props = $props();

  const W = 600;
  const H = 220;
  const pad = { left: 44, right: 16, top: 14, bottom: 28 };
  const frame = { width: W - pad.left - pad.right, height: H - pad.top - pad.bottom };

  // FlowDay.counts は日ごとなので、stackPaths が受ける層ごとの並びに入れ替える。
  const series = $derived(layers.map((_, i) => flow.map((d) => d.counts[i] ?? 0)));
  const peak = $derived(Math.max(0, ...flow.map((d) => d.counts.reduce((a, b) => a + b, 0))));
  const max = $derived(niceMax(peak));
  const paths = $derived(stackPaths(series, max, frame));
  const yOf = (v: number) => linePoints([v], max, frame)[0]?.y ?? 0;
</script>

<figure>
  <svg viewBox="0 0 {W} {H}" role="img" aria-label="累積フロー図">
    <g transform="translate({pad.left} {pad.top})">
      {#each ticks(max) as tick (tick)}
        <line class="grid" x1="0" x2={frame.width} y1={yOf(tick)} y2={yOf(tick)} />
        <text class="axis" x="-6" y={yOf(tick)} text-anchor="end" dominant-baseline="middle">
          {tick}
        </text>
      {/each}
      {#each layers as layer, i (layer.key)}
        <path d={paths[i]} fill={layerFill(layer)} />
      {/each}
      <text class="axis" x="0" y={frame.height + 18}>
        {formatDay(flow[0]?.date ?? "", thisYear)}
      </text>
      <text class="axis" x={frame.width} y={frame.height + 18} text-anchor="end">
        {formatDay(flow.at(-1)?.date ?? "", thisYear)}
      </text>
    </g>
  </svg>
  <figcaption>
    {#each layers as layer (layer.key)}
      <span><i class="key" style="background: {layerFill(layer)}"></i>{layer.name}</span>
    {/each}
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
  path {
    stroke: var(--surface);
    stroke-width: 0.5;
  }
  figcaption {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 16px;
    font-size: 12px;
    color: var(--muted);
  }
  .key {
    display: inline-block;
    width: 10px;
    height: 10px;
    margin-right: 6px;
    border-radius: 2px;
  }
</style>
