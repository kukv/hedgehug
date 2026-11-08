<script lang="ts">
  import { colorToken, STATUS_COLORS } from "../../core/schema-edit";

  // The color of a status or a kind: one of the palette, a free color (#rrggbb), or auto. original is
  // the value the schema has now; one that is neither in the palette nor a #rrggbb stays a choice
  // ("カスタム") so that it is kept unless another is picked.
  type Props = { value: string; original?: string; label: string };
  let { value = $bindable(), original = "", label }: Props = $props();

  const uid = $props.id();
  // input[type=color] only takes a 6-digit hex.
  const isFree = (v: string) => /^#[0-9a-f]{6}$/i.test(v);
  // What the free color field shows: the color the schema has, else a blue to start from.
  let free = $state(isFree(value) ? value.toLowerCase() : "#3451d1");
  const custom = $derived(
    original !== "" && !isFree(original) && !STATUS_COLORS.some((c) => c.value === original),
  );
  const options = $derived([
    ...(custom ? [{ value: original, label: `カスタム（${original}）` }] : []),
    { value: "", label: "自動" },
    ...STATUS_COLORS,
  ]);
  const swatch = (v: string): string | undefined => (v === "" ? undefined : colorToken(v));
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
  <div class="opt">
    <label class="free-label">
      <input
        type="radio"
        name="{uid}-color"
        checked={isFree(value)}
        onchange={() => (value = free)}
      >
      <span class="dot" style:background={free}></span>
      自由な色
    </label>
    <input
      type="color"
      class="free"
      aria-label="色を選ぶ"
      value={free}
      oninput={(e) => {
        free = e.currentTarget.value;
        value = free;
      }}
    >
  </div>
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
  .opt:has(input[type="radio"]:focus-visible) {
    outline: 2px solid var(--accent);
  }
  input[type="radio"] {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .free-label {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
    cursor: pointer;
  }
  .free {
    width: 24px;
    height: 20px;
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
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
