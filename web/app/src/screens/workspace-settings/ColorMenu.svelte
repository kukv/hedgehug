<script lang="ts">
  import { colors } from "@hedgehug/core";
  import { Menu } from "@hedgehug/ui";

  // 色のボタンと 18 色のメニュー(03「色」)。
  interface Props {
    color: string;
    name: string;
    disabled?: boolean;
    title?: string | undefined;
    onselect: (color: string) => void;
  }

  let { color, name, disabled = false, title, onselect }: Props = $props();

  let at = $state<{ x: number; y: number } | null>(null);

  function open(event: MouseEvent) {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    at = { x: rect.left, y: rect.bottom + 4 };
  }
</script>

<button
  type="button"
  class="swatch"
  aria-label="{name}の色"
  {disabled}
  title={title ?? colors.find((c) => c.id === color)?.label}
  style:background="var(--c-{color}-dot)"
  onclick={open}
></button>
<Menu
  open={at !== null}
  x={at?.x ?? 0}
  y={at?.y ?? 0}
  items={colors.map((c) => ({
    id: c.id,
    label: c.label,
    checked: c.id === color,
  }))}
  onselect={(id) => {
    at = null;
    if (id !== color) onselect(id);
  }}
  onclose={() => (at = null)}
/>

<style>
  .swatch {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    border: 1px solid var(--line-input);
    padding: 0;
    flex: none;
  }
  .swatch:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
