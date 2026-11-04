<script lang="ts" module>
  import type { StoreTone } from "../../core/stores";

  // The palette of config.yaml (StoreColors in the server), with the words the screen uses for each.
  export const STORE_COLORS: { value: Exclude<StoreTone, "readonly">; label: string }[] = [
    { value: "teal", label: "ティール" },
    { value: "orange", label: "オレンジ" },
    { value: "purple", label: "パープル" },
    { value: "sky", label: "スカイ" },
    { value: "pink", label: "ピンク" },
    { value: "olive", label: "オリーブ" },
    { value: "brown", label: "ブラウン" },
  ];
</script>

<script lang="ts">
  import Menu, { type MenuItem } from "../ui/Menu.svelte";
  import StoreDot from "./StoreDot.svelte";

  // The seven colours and "自動". current is the saved colour ("" for automatic). onselect gets the
  // colour, or "" for automatic, as PATCH /api/stores/{name} takes it.
  type Props = {
    anchor: HTMLElement | null;
    current: string;
    label: string;
    onselect: (color: string) => void;
    onclose: () => void;
  };
  let { anchor, current, label, onselect, onclose }: Props = $props();

  const AUTO = "auto";
  const items = $derived<MenuItem[]>([
    ...STORE_COLORS.map((c) => ({ value: c.value, label: c.label, checked: c.value === current })),
    { value: AUTO, label: "自動", checked: current === "" },
  ]);
</script>

{#snippet entry(it: MenuItem)}
  <StoreDot tone={it.value === AUTO ? "auto" : (it.value as StoreTone)} />
  <span class="name">{it.label}</span>
  {#if it.checked}
    <span class="mark">選択中</span>
  {/if}
{/snippet}

<Menu
  {anchor}
  {items}
  {label}
  item={entry}
  onselect={(v) => onselect(v === AUTO ? "" : v)}
  {onclose}
/>

<style>
  .name {
    flex: 1;
  }
  .mark {
    color: var(--accent-ink);
    font-size: var(--fs-11);
  }
</style>
