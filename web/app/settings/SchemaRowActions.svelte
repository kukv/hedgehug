<script lang="ts">
  import Button from "../ui/Button.svelte";
  import type { SchemaAction } from "./schema-action";

  // The buttons of a row: edit the definition (its name too), merge it into another (a status only:
  // properties are not merged), delete it. used is how many tasks use the definition, and a definition
  // that is used or the default cannot be deleted.
  let {
    target,
    old,
    names,
    off,
    onaction,
    used,
    isDefault = false,
  }: {
    target: "status" | "prop";
    old: string;
    names: string[];
    off: string | null;
    onaction: (a: SchemaAction) => void;
    used?: number;
    isDefault?: boolean;
  } = $props();

  const subject = $derived(old);
  const deleteWhy = $derived(
    off ??
      ((used ?? 0) > 0
        ? target === "status"
          ? "タスクが使っているため削除できません。マージで別のステータスへ移してから削除してください"
          : "タスクが使っているため削除できません。使っているタスクの値を外してから削除してください"
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
  onclick={() => onaction({ kind: "edit", target, name: old })}
>
  編集
</Button>
{#if target === "status" && names.length > 1}
  <Button
    size="sm"
    aria-label="マージ（{subject}）"
    disabled={off !== null}
    why={off ?? undefined}
    onclick={() => onaction({ kind: "migrate", target: "status", old, names })}
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
