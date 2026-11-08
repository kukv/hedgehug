<script lang="ts">
  import { KINDS } from "../../core/kinds";

  // One select per kind for its default template ("<種類> の既定"): the Store's settings and a
  // project's override both use it. none is the text of the empty choice for a kind.
  type Props = {
    names: string[];
    values: Record<string, string>;
    disabled: boolean;
    none: (kind: string) => string;
    onchange: (kind: string, name: string) => void;
  };
  let { names, values, disabled, none, onchange }: Props = $props();
</script>

<div class="defaults">
  {#each KINDS as k (k.name)}
    <label>
      <span>{k.label} の既定</span>
      <select
        value={values[k.name] ?? ""}
        {disabled}
        onchange={(e) => onchange(k.name, e.currentTarget.value)}
      >
        <option value="">{none(k.name)}</option>
        {#each names as n (n)}
          <option value={n}>{n}</option>
        {/each}
      </select>
    </label>
  {/each}
</div>

<style>
  .defaults {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    max-width: 420px;
  }
  label {
    display: flex;
    gap: var(--space-12);
    align-items: center;
    justify-content: space-between;
  }
  select {
    min-height: 30px;
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
  }
</style>
