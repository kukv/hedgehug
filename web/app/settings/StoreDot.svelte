<script lang="ts">
  import type { StoreTone } from "../../core/stores";

  // The colour of a Store as a dot. "auto" is the ring that stands for "the colour of its place in the list".
  let { tone }: { tone: StoreTone | "auto" } = $props();
</script>

<span class="dot" data-tone={tone} aria-hidden="true"></span>

<style>
  .dot {
    flex: 0 0 auto;
    box-sizing: border-box;
    width: var(--space-12);
    height: var(--space-12);
    border-radius: var(--radius-6);
    background: var(--dot);
  }
  .dot[data-tone="auto"] {
    border: 1.5px dashed var(--muted-2);
    background: transparent;
  }
  .dot[data-tone="teal"] {
    --dot: var(--store-teal-dot);
  }
  .dot[data-tone="orange"] {
    --dot: var(--store-orange-dot);
  }
  .dot[data-tone="purple"] {
    --dot: var(--store-purple-dot);
  }
  .dot[data-tone="sky"] {
    --dot: var(--store-sky-dot);
  }
  .dot[data-tone="pink"] {
    --dot: var(--store-pink-dot);
  }
  .dot[data-tone="olive"] {
    --dot: var(--store-olive-dot);
  }
  .dot[data-tone="brown"] {
    --dot: var(--store-brown-dot);
  }
  .dot[data-tone="readonly"] {
    --dot: var(--store-readonly-dot);
  }
</style>
