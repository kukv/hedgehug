<script lang="ts">
  import { checklistItems } from "./checklist-items";

  let {
    body,
    readonly = false,
    ontoggle,
  }: { body: string; readonly?: boolean; ontoggle: (line: number) => void } = $props();

  const items = $derived(checklistItems(body));
  const done = $derived(items.filter((i) => i.checked).length);
</script>

{#if items.length > 0}
  <section aria-labelledby="checklist-h">
    <h2 id="checklist-h">チェックリスト <span class="count">{done}/{items.length}</span></h2>
    <ul>
      {#each items as it (it.line)}
        <li>
          <label class:done={it.checked}>
            <input
              type="checkbox"
              checked={it.checked}
              disabled={readonly}
              onchange={() => ontoggle(it.line)}
            >
            <span>{it.text}</span>
          </label>
        </li>
      {/each}
    </ul>
    <p class="hint">
      チェックはここで切り替えられます。別の行どうしなら、同時に切り替えても衝突しません。
    </p>
  </section>
{/if}

<style>
  h2 {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .count {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: var(--space-4);
  }
  label {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  label.done span {
    color: var(--muted);
    text-decoration: line-through;
  }
  .hint {
    margin: var(--space-8) 0 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
</style>
