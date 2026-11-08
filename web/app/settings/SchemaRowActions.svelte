<script lang="ts">
  import Button from "../ui/Button.svelte";
  import type { MigrateTarget } from "./MigrateDialog.svelte";
  import type { SchemaAction } from "./schema-action";

  // The buttons of a row: change the name of a definition, or merge it into another.
  let {
    target,
    old,
    names,
    prop,
    off,
    onaction,
  }: {
    target: MigrateTarget;
    old: string;
    names: string[];
    prop?: string;
    off: string | null;
    onaction: (a: SchemaAction) => void;
  } = $props();

  const subject = $derived(prop ? `${prop} の ${old}` : old);
</script>

<Button
  size="sm"
  aria-label="名前…（{subject}）"
  disabled={off !== null}
  why={off ?? undefined}
  onclick={() => onaction({ kind: "migrate", target, prop, old, mode: "rename", names })}
>
  名前…
</Button>
{#if names.length > 1}
  <Button
    size="sm"
    aria-label="マージ…（{subject}）"
    disabled={off !== null}
    why={off ?? undefined}
    onclick={() => onaction({ kind: "migrate", target, prop, old, mode: "merge", names })}
  >
    マージ…
  </Button>
{/if}
