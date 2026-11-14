<script lang="ts">
  import { bars, niceMax, type ReportUnit, ticks, type VelocityBar } from "@hedgehug/core";
  import { num } from "./format";

  // ベロシティの棒(群ごとに「計画」「完了」)。棒に値を書き、凡例も文字で書く(色だけに頼らない)。
  interface Props {
    items: VelocityBar[];
    unit: ReportUnit;
  }

  let { items, unit }: Props = $props();

  const W = 600;
  const H = 220;
  const pad = { left: 44, right: 12, top: 18, bottom: 28 };
  const frame = { width: W - pad.left - pad.right, height: H - pad.top - pad.bottom };

  const groups = $derived(items.map((b) => [b.planned, b.done]));
  const max = $derived(niceMax(Math.max(0, ...groups.flat())));
  const drawn = $derived(bars(groups, max, frame));
  const slot = $derived(frame.width / items.length);
  const yOf = (v: number) => frame.height - (v / max) * frame.height;
</script>

<figure>
  <svg viewBox="0 0 {W} {H}" role="img" aria-label="ベロシティ">
    <g transform="translate({pad.left} {pad.top})">
      {#each ticks(max) as tick (tick)}
        <line class="grid" x1="0" x2={frame.width} y1={yOf(tick)} y2={yOf(tick)} />
        <text class="axis" x="-6" y={yOf(tick)} text-anchor="end" dominant-baseline="middle">
          {tick}
        </text>
      {/each}
      {#each drawn as group, g (items[g]?.sprint.id)}
        {#each group as bar, i (i)}
          <rect
            class={i === 0 ? "planned" : "done"}
            x={bar.x}
            y={bar.y}
            width={bar.width}
            height={bar.height}
          />
          <text class="value" x={bar.x + bar.width / 2} y={bar.y - 4} text-anchor="middle">
            {num(groups[g]?.[i] ?? 0, unit)}
          </text>
        {/each}
        <text class="axis" x={slot * g + slot / 2} y={frame.height + 18} text-anchor="middle">
          {items[g]?.sprint.name}
        </text>
      {/each}
    </g>
  </svg>
  <figcaption>
    <span><i class="key planned"></i>計画</span>
    <span><i class="key done"></i>完了</span>
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
    font-size: 11px;
    fill: var(--ink);
  }
  .planned {
    fill: var(--muted-2);
    background: var(--muted-2);
  }
  .done {
    fill: var(--accent);
    background: var(--accent);
  }
  figcaption {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: var(--muted);
  }
  .key {
    display: inline-block;
    width: 10px;
    height: 10px;
    margin-right: 6px;
  }
</style>
