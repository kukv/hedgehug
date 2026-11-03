<script lang="ts">
  import { tick } from "svelte";
  import type { SchemaView } from "../../core/schema";
  import { addValues, type Lists, removeValue } from "../../core/scoped";
  import Button from "../ui/Button.svelte";

  // One list of constraints of the workflow: what the Store has (it cannot be taken off), what the
  // project added (it can), and the form that adds. mode says which list it is: the statuses a
  // status may move to, or the fields a status needs.
  type Props = {
    mode: "transition" | "required";
    statuses: NonNullable<SchemaView["statuses"]>;
    fields: string[];
    mine: Lists;
    fromStore: Lists;
    busy: boolean;
    onchange: (next: Lists) => void;
  };
  let { mode, statuses, fields, mine, fromStore, busy, onchange }: Props = $props();

  const transition = $derived(mode === "transition");
  const heading = $derived(transition ? "遷移の制約" : "必須の項目");
  const formName = $derived(transition ? "遷移の制約を足す" : "必須の項目を足す");
  const picksName = $derived(transition ? "行ける先" : "必要な項目");
  const selectName = $derived(transition ? "遷移元のステータス" : "必須にするステータス");
  const placeholder = $derived(transition ? "遷移元を選ぶ" : "ステータスを選ぶ");

  let pick = $state("");
  let chosen = $state<string[]>([]);
  let select: HTMLSelectElement | undefined = $state();
  let list: HTMLUListElement | undefined = $state();

  const labelOf = (name: string) => statuses.find((s) => s.name === name)?.label || name;
  // What a value is called in a row: a status by its label, a field by its name.
  const shownAs = (v: string) => (transition ? labelOf(v) : v);
  const options = $derived(
    transition
      ? statuses
          .filter((s) => s.name !== pick)
          .map((s) => ({ value: s.name, label: s.label || s.name }))
      : fields.map((f) => ({ value: f, label: f })),
  );
  const items = $derived([
    ...Object.entries(fromStore).map(([key, values]) => ({ key, values, own: false })),
    ...Object.entries(mine).map(([key, values]) => ({ key, values, own: true })),
  ]);

  async function add() {
    const order = transition ? statuses.map((s) => s.name) : fields;
    onchange(
      addValues(
        mine,
        pick,
        order.filter((v) => chosen.includes(v)),
      ),
    );
    pick = "";
    chosen = [];
    await tick();
    select?.focus();
  }

  async function remove(key: string, value: string) {
    onchange(removeValue(mine, key, value));
    await tick();
    // The list is gone when that was its last row: the form is the next place.
    (list ?? select)?.focus();
  }
</script>

<h3>{heading}</h3>
{#if items.length === 0}
  <p class="msg">まだありません</p>
{:else}
  <ul class="rows" aria-label={heading} tabindex="-1" bind:this={list}>
    {#each items as it (`${it.own ? "mine" : "store"}:${it.key}`)}
      <li class="row">
        <span class="from">{labelOf(it.key)}</span>
        <span class="arrow" aria-hidden="true">{transition ? "→" : "に必要"}</span>
        <span class="chips">
          {#each it.values as v (v)}
            <span class="chip">
              {shownAs(v)}
              {#if it.own}
                <button
                  type="button"
                  class="drop"
                  aria-label="{shownAs(v)}を外す"
                  disabled={busy}
                  onclick={() => void remove(it.key, v)}
                >
                  ×
                </button>
              {/if}
            </span>
          {/each}
        </span>
        <span class="src" class:own={it.own}>{it.own ? "このプロジェクトで足した" : "Store"}</span>
      </li>
    {/each}
  </ul>
{/if}
<fieldset class="add">
  <legend>{formName}</legend>
  <select
    aria-label={selectName}
    bind:this={select}
    disabled={busy}
    value={pick}
    onchange={(e) => {
      pick = e.currentTarget.value;
      // A status cannot be its own target; a field of the same name is another thing.
      if (transition) chosen = chosen.filter((v) => v !== pick);
    }}
  >
    <option value="">{placeholder}</option>
    {#each statuses as s (s.name)}
      <option value={s.name}>{s.label || s.name}</option>
    {/each}
  </select>
  <fieldset class="picks">
    <legend>{picksName}</legend>
    {#each options as o (o.value)}
      <label class="pick">
        <input
          type="checkbox"
          checked={chosen.includes(o.value)}
          disabled={busy}
          onchange={(e) =>
            (chosen = e.currentTarget.checked
              ? [...chosen, o.value]
              : chosen.filter((v) => v !== o.value))}
        >
        {o.label}
      </label>
    {/each}
  </fieldset>
  <Button
    size="sm"
    disabled={busy || pick === "" || chosen.length === 0}
    onclick={() => void add()}
  >
    {formName}
  </Button>
</fieldset>

<style>
  h3 {
    margin: var(--space-8) 0 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .rows {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-8) 0;
    border-bottom: 1px solid var(--line);
    font-size: var(--fs-13);
  }
  .from {
    font-weight: 600;
  }
  .arrow {
    color: var(--muted);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: 2px var(--space-8);
    border-radius: var(--radius-pill);
    background: var(--sunken);
  }
  .drop {
    border: 0;
    background: none;
    color: var(--muted);
    font: inherit;
    cursor: pointer;
  }
  .drop:hover:not(:disabled) {
    color: var(--danger);
  }
  .src {
    margin-left: auto;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .src.own {
    color: var(--accent-ink);
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-12);
    min-width: 0;
    margin: 0;
    padding: var(--space-8) 0 0;
    border: 0;
  }
  legend {
    padding: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .picks {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8) var(--space-16);
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  .pick {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    font-size: var(--fs-13);
  }
  select {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
</style>
