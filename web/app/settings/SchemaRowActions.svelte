<script lang="ts">
  import Button from "../ui/Button.svelte";
  import type { MigrateTarget } from "./MigrateDialog.svelte";
  import type { SchemaAction } from "./schema-action";

  // The buttons of a row: edit the definition, change its name, merge it into another, delete it, add a
  // value to it. edit is the target of the edit dialog (a value has none); used is how many tasks use the
  // definition, and a definition that is used or the default cannot be deleted.
  let {
    target,
    old,
    names,
    prop,
    off,
    onaction,
    edit,
    used,
    isDefault = false,
    addValue = false,
  }: {
    target: MigrateTarget;
    old: string;
    names: string[];
    prop?: string;
    off: string | null;
    onaction: (a: SchemaAction) => void;
    edit?: "status" | "kind" | "prop" | "relation";
    used?: number;
    isDefault?: boolean;
    addValue?: boolean;
  } = $props();

  const subject = $derived(prop ? `${prop} の ${old}` : old);
  const deleteWhy = $derived(
    off ??
      ((used ?? 0) > 0
        ? "タスクが使っているため削除できません。マージ… で別の定義へ移してから削除してください"
        : isDefault
          ? "既定は削除できません。別のものを既定にしてから削除してください"
          : null),
  );
</script>

{#if edit}
  <Button
    size="sm"
    aria-label="編集…（{subject}）"
    disabled={off !== null}
    why={off ?? undefined}
    onclick={() => onaction({ kind: "edit", target: edit, name: old })}
  >
    編集…
  </Button>
{/if}
{#if addValue}
  <Button
    size="sm"
    aria-label="値を追加…（{subject}）"
    disabled={off !== null}
    why={off ?? undefined}
    onclick={() => onaction({ kind: "add-value", prop: old })}
  >
    値を追加…
  </Button>
{/if}
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
<Button
  size="sm"
  aria-label="削除…（{subject}）"
  disabled={deleteWhy !== null}
  why={deleteWhy ?? undefined}
  onclick={() => onaction({ kind: "delete", target, name: old, prop })}
>
  削除…
</Button>
