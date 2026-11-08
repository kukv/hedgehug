<script lang="ts">
  import { STATUS_COLORS, statusColor } from "../../core/schema-edit";

  // The color of a status: one of the palette, or auto. original is the value the schema has now; one
  // that is not in the palette stays a choice ("カスタム") so that it is kept unless another is picked.
  type Props = { value: string; original?: string; label: string };
  let { value = $bindable(), original = "", label }: Props = $props();

  const uid = $props.id();
  const custom = $derived(original !== "" && !STATUS_COLORS.some((c) => c.value === original));
  const options = $derived([
    ...(custom ? [{ value: original, label: `カスタム（${original}）` }] : []),
    { value: "", label: "自動" },
    ...STATUS_COLORS,
  ]);
  const swatch = (v: string): string | undefined => (v === "" ? undefined : statusColor(v));
</script>

<div class="group" role="radiogroup" aria-label={label}>
  {#each options as o (o.value)}
    <label class="opt">
      <input
        type="radio"
        name="{uid}-color"
        checked={value === o.value}
        onchange={() => (value = o.value)}
      >
      <span
        class="dot"
        class:auto={swatch(o.value) === undefined}
        style:background={swatch(o.value)}
      ></span>
      {o.label}
    </label>
  {/each}
</div>

<style>
  .group {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
  }
  .opt {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
    min-height: var(--control-h-sm, 28px);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .opt:has(input:checked) {
    border-color: var(--accent);
    font-weight: 600;
  }
  .opt:has(input:focus-visible) {
    outline: 2px solid var(--accent);
  }
  input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }
  .auto {
    border: 1px dashed var(--muted);
  }
</style>
