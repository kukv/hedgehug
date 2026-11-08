<script lang="ts">
  import { KIND_ICONS } from "../../core/schema-edit";
  import KindIcon from "../icons/KindIcon.svelte";

  // The icon of a kind: one we draw, or auto (the shape of its role). original is the value the schema
  // has now; one we do not draw stays a choice ("カスタム") so that it is kept unless another is picked.
  type Props = { value: string; original?: string; role: string; label: string };
  let { value = $bindable(), original = "", role, label }: Props = $props();

  const uid = $props.id();
  const custom = $derived(original !== "" && !KIND_ICONS.some((i) => i.value === original));
  const options = $derived([
    ...(custom ? [{ value: original, label: `カスタム（${original}）` }] : []),
    { value: "", label: "自動" },
    ...KIND_ICONS,
  ]);
</script>

<div class="group" role="radiogroup" aria-label={label}>
  {#each options as o (o.value)}
    <label class="opt">
      <input
        type="radio"
        name="{uid}-icon"
        checked={value === o.value}
        onchange={() => (value = o.value)}
      >
      <KindIcon kind={o.value} {role} icon={o.value} size={14} />
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
</style>
