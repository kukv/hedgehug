<script lang="ts">
  import type { Resolved, Route } from "@hedgehug/core";
  import { Brand } from "@hedgehug/ui";
  import type { Links } from "../links";

  // サイドバーの上端(04「サイドバー」)。アプリの名前と設定の歯車。歯車は設定の「一般」を開く(G1-13)。
  interface Props {
    current: Resolved | null;
    links: Links;
  }

  let { current, links }: Props = $props();

  const settings: Route = { name: "settings", tab: "general" };
</script>

<div class="top">
  <Brand />
  <a
    class="gear"
    href={links.href(settings)}
    aria-label="設定"
    title="設定"
    aria-current={current?.name === "settings" ? "page" : undefined}
    onclick={(e) => links.click(e, settings)}
    onauxclick={(e) => links.click(e, settings)}
  >
    <svg width="16" height="16" viewBox="0 0 24 24">
      <title>設定</title>
      <circle cx="12" cy="12" r="3" />
      <path
        d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
      />
    </svg>
  </a>
</div>

<style>
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .gear {
    display: inline-grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: 8px;
    color: var(--muted);
  }
  .gear:hover,
  .gear[aria-current="page"] {
    background: var(--hover);
    color: var(--ink);
  }
  .gear svg {
    stroke: currentColor;
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
