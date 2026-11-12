<script lang="ts">
  import type { ValueField } from "@hedgehug/core";
  import { Checkbox, Input, Select } from "@hedgehug/ui";

  // テンプレートの値の欄(03「持てるもの」)。値は文字、複数は配列、空は "" か []。
  interface Props {
    fields: ValueField[];
    values: Record<string, string | string[]>;
    disabled: boolean;
    title?: string;
  }

  let { fields, values = $bindable(), disabled, title }: Props = $props();

  const one = (id: string) => {
    const v = values[id];
    return typeof v === "string" ? v : "";
  };
  const many = (id: string) => {
    const v = values[id];
    return Array.isArray(v) ? v : [];
  };
  const set = (id: string, v: string) => {
    values[id] = v;
  };
  const toggle = (id: string, value: string, on: boolean) => {
    const rest = many(id).filter((x) => x !== value);
    values[id] = on ? [...rest, value] : rest;
  };
</script>

{#each fields as f (f.propertyId)}
  <div class="field">
    <span class="k">{f.label}</span>
    {#if f.input === "select"}
      <Select
        label={f.label}
        options={f.options}
        {disabled}
        {title}
        bind:value={() => one(f.propertyId), (v) => set(f.propertyId, v)}
      />
    {:else if f.input === "multi"}
      <div class="multi">
        {#each f.options as o (o.value)}
          <Checkbox
            label={o.label}
            {disabled}
            {title}
            bind:checked={
              () => many(f.propertyId).includes(o.value),
              (on) => toggle(f.propertyId, o.value, on)
            }
          />
        {/each}
      </div>
    {:else if f.input === "number"}
      <!-- ui の Input は text と date だけなので、数はネイティブの欄で受ける(値は文字のまま持ち、送るときに数へ直す)。 -->
      <input
        class="number"
        type="number"
        step="any"
        aria-label={f.label}
        {disabled}
        {title}
        value={one(f.propertyId)}
        oninput={(e) => set(f.propertyId, e.currentTarget.value)}
      >
    {:else}
      <Input
        label={f.label}
        type={f.input === "date" ? "date" : "text"}
        {disabled}
        {title}
        bind:value={() => one(f.propertyId), (v) => set(f.propertyId, v)}
      />
    {/if}
  </div>
{/each}

<style>
  .field {
    display: grid;
    grid-template-columns: 120px 1fr;
    align-items: center;
    gap: 8px;
  }
  .k {
    font-size: 12.5px;
    color: var(--ink-2);
  }
  /* 見た目は ui の Input に合わせる。 */
  .number {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .number:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .number:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .multi {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 14px;
  }
</style>
