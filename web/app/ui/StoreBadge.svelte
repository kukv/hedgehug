<script lang="ts">
  import type { StoreTone } from "../../core/stores";
  import { core } from "../context";
  import { useReadOnly } from "../read-only";

  type Props = { store: string; tone: StoreTone; compact?: boolean };
  let { store, tone, compact = false }: Props = $props();

  // tone is the colour only: a Store that is not known is grey too, so read-only is asked of the directory.
  const readOnly = useReadOnly(core().stores, () => store);
  const marked = $derived(readOnly.current !== null);
</script>

<span
  class="badge"
  class:compact
  data-tone={tone}
  title={marked ? `${store}（閲覧のみ）` : `Store: ${store}`}
>
  <span class="dot"></span>
  {store}
  {#if marked}
    <span class="sr">（閲覧のみ）</span>
  {/if}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    max-width: 100%;
    height: 22px;
    padding: 0 var(--space-8);
    border-radius: 999px;
    background: var(--bg);
    color: var(--fg);
    font-size: var(--fs-12);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    position: relative;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
  .compact {
    height: 16px;
    padding: 0 var(--space-6);
    gap: var(--space-4);
    font-size: 10px;
  }
  .dot {
    flex: 0 0 auto;
    width: 6px;
    height: 6px;
    border-radius: 2px;
    background: var(--dot);
  }
  .compact .dot {
    width: 5px;
    height: 5px;
  }
  .badge[data-tone="teal"] {
    --fg: var(--store-teal-fg);
    --bg: var(--store-teal-bg);
    --dot: var(--store-teal-dot);
  }
  .badge[data-tone="orange"] {
    --fg: var(--store-orange-fg);
    --bg: var(--store-orange-bg);
    --dot: var(--store-orange-dot);
  }
  .badge[data-tone="purple"] {
    --fg: var(--store-purple-fg);
    --bg: var(--store-purple-bg);
    --dot: var(--store-purple-dot);
  }
  .badge[data-tone="sky"] {
    --fg: var(--store-sky-fg);
    --bg: var(--store-sky-bg);
    --dot: var(--store-sky-dot);
  }
  .badge[data-tone="pink"] {
    --fg: var(--store-pink-fg);
    --bg: var(--store-pink-bg);
    --dot: var(--store-pink-dot);
  }
  .badge[data-tone="olive"] {
    --fg: var(--store-olive-fg);
    --bg: var(--store-olive-bg);
    --dot: var(--store-olive-dot);
  }
  .badge[data-tone="brown"] {
    --fg: var(--store-brown-fg);
    --bg: var(--store-brown-bg);
    --dot: var(--store-brown-dot);
  }
  .badge[data-tone="readonly"] {
    --fg: var(--store-readonly-fg);
    --bg: var(--store-readonly-bg);
    --dot: var(--store-readonly-dot);
  }
</style>
