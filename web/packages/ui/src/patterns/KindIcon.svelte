<script lang="ts">
  import type { KindName } from "../types";

  interface Props {
    kind: KindName;
  }

  let { kind }: Props = $props();

  const glyphs: Record<KindName, string> = {
    Epic: "E",
    Story: "S",
    Task: "T",
    Bug: "B",
    Step: "✓",
    Project: "P",
  };
</script>

<span
  class="kind"
  role="img"
  aria-label={kind}
  title={kind}
  style="--k: var(--k-{kind.toLowerCase()})"
>
  {glyphs[kind]}
</span>

<style>
  .kind {
    width: 18px;
    height: 18px;
    border-radius: 5px;
    display: inline-grid;
    place-items: center;
    font-size: 10.5px;
    font-weight: 600;
    color: var(--surface);
    background: var(--k);
    flex: none;
    font-family: var(--font-mono);
  }
</style>
