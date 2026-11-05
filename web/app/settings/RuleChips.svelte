<script lang="ts">
  import { onLabel, setLabel, whenLabel } from "../../core/automations";
  import type { Rule } from "../../core/github";
  import type { SchemaView } from "../../core/schema";

  // What a rule does: the event, its condition, and what it sets.
  let { rule, schema }: { rule: Rule; schema: SchemaView } = $props();
</script>

<span class="chips">
  <span class="chip">{onLabel(rule)}</span>
  {#if rule.when}
    <span class="chip">{whenLabel(rule.when)}</span>
  {/if}
  <span aria-hidden="true">→</span>
  <span class="chip set">{setLabel(rule, schema)}</span>
</span>

<style>
  .chips {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-6);
    color: var(--muted);
  }
  .chip {
    padding: 2px var(--space-8);
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink);
    font-size: var(--fs-13);
  }
  .set {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
</style>
