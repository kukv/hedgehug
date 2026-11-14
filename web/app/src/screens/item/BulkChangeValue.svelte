<script lang="ts">
  import type { BulkChangeForm } from "@hedgehug/core";
  import { Button, Input, ValueMenu } from "@hedgehug/ui";
  import { bulkChange } from "../../wireActions";

  // 一括変更ダイアログの「値」の入力(F2b2-22)。項目で入力の種類が変わる。
  // 「分類で変更」の選択肢は group を持つので、ui の Select ではなく <select> をここに書く(optgroup が要る)。
  interface Props {
    form: BulkChangeForm;
  }

  let { form }: Props = $props();
  let menu = $state<{ x: number; y: number } | null>(null);

  const systems = $derived(form.field === "systemsAdd" || form.field === "systemsRemove");
  const date = $derived(form.field === "start" || form.field === "end");
  const number = $derived(form.field === "points" || form.field === "estimate");
  const plain = $derived(form.options.filter((o) => !o.group));
  const grouped = $derived(form.options.filter((o) => o.group));

  function openMenu(event: MouseEvent) {
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    menu = { x: r.left, y: r.bottom + 4 };
  }
</script>

{#if systems}
  <Button disabled={form.busy} onclick={openMenu}>
    {form.values.length > 0 ? form.values.join(", ") : "選択"}
  </Button>
  <ValueMenu
    open={menu !== null}
    x={menu?.x ?? 0}
    y={menu?.y ?? 0}
    label="値"
    options={form.options.map((o) => ({
      id: o.value,
      label: o.label,
      checked: form.values.includes(o.value),
    }))}
    multi
    onselect={bulkChange.toggleValue}
    onclose={() => (menu = null)}
  />
{:else if date || number}
  <Input
    label="値"
    type={date ? "date" : "text"}
    inputmode={number ? "decimal" : undefined}
    value={form.text}
    disabled={form.busy}
    oninput={bulkChange.setText}
  />
{:else}
  <select
    class="select"
    aria-label="値"
    disabled={form.busy}
    value={form.option}
    onchange={(event) => bulkChange.setOption(event.currentTarget.value)}
  >
    {#each plain as o (o.value)}
      <option value={o.value}>{o.label}</option>
    {/each}
    {#if grouped.length > 0}
      <optgroup label={grouped[0]?.group}>
        {#each grouped as o (o.value)}
          <option value={o.value}>{o.label}</option>
        {/each}
      </optgroup>
    {/if}
  </select>
{/if}

<style>
  .select {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .select:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .select:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
