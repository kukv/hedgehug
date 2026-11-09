<script lang="ts">
  import Button from "../ui/Button.svelte";
  import type { MigrateTarget } from "./MigrateDialog.svelte";
  import type { SchemaAction } from "./schema-action";

  // The buttons of a row: edit the definition (its name too), merge it into another, delete it. edit is
  // the target of the edit dialog; used is how many tasks use the definition, and a definition that is
  // used or the default cannot be deleted.
  let {
    target,
    old,
    names,
    off,
    onaction,
    edit,
    used,
    isDefault = false,
  }: {
    target: MigrateTarget;
    old: string;
    names: string[];
    off: string | null;
    onaction: (a: SchemaAction) => void;
    edit: "status" | "prop";
    used?: number;
    isDefault?: boolean;
  } = $props();

  const subject = $derived(old);
  const deleteWhy = $derived(
    off ??
      ((used ?? 0) > 0
        ? "タスクが使っているため削除できません。マージで別の定義へ移してから削除してください"
        : isDefault
          ? "既定は削除できません。別のものを既定にしてから削除してください"
          : null),
  );
</script>

<Button
  size="sm"
  aria-label="編集（{subject}）"
  disabled={off !== null}
  why={off ?? undefined}
  onclick={() => onaction({ kind: "edit", target: edit, name: old })}
>
  編集
</Button>
{#if names.length > 1}
  <Button
    size="sm"
    aria-label="マージ（{subject}）"
    disabled={off !== null}
    why={off ?? undefined}
    onclick={() => onaction({ kind: "migrate", target, old, names })}
  >
    マージ
  </Button>
{/if}
<Button
  size="sm"
  aria-label="削除（{subject}）"
  disabled={deleteWhy !== null}
  why={deleteWhy ?? undefined}
  onclick={() => onaction({ kind: "delete", target, name: old })}
>
  削除
</Button>
