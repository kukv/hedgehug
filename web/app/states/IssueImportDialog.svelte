<script lang="ts">
  import type { ImportTarget } from "../../core/github";
  import { core } from "../context";
  import { useDisplayId } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import IssueImportFields from "./IssueImportFields.svelte";
  import { useIssueImport } from "./issue-import.svelte";

  // Takes in the open issues with a label under a project.

  let { target, onclose }: { target: ImportTarget; onclose: () => void } = $props();

  const did = useDisplayId(core().stores);
  const form = useIssueImport(
    () => target.store,
    () => ({ parent: target.parent, parentId: did.current(target.store, target.parentId) }),
    () => onclose(),
  );
</script>

<Dialog
  open
  title="Issue から取り込む"
  width={560}
  cli={form.cli}
  cliDisabled={!form.ready}
  {onclose}
>
  <div class="form">
    <IssueImportFields store={target.store} {form} />
    {#if form.counted}
      <p class="count">
        open の Issue {form.counted.count} 件 · うち {form.counted.skipped}
        件はリンク済みのため飛ばします
      </p>
    {/if}
    {#if form.error}
      <FormError message={form.error} />
    {/if}
  </div>
  {#snippet footer()}
    <Button disabled={form.busy} onclick={onclose}>キャンセル</Button>
    {#if form.counted}
      <Button
        variant="confirm"
        disabled={form.busy || form.counted.count === 0}
        onclick={() => void form.run(false)}
      >
        {form.counted.count}
        件を mirror で取り込む
      </Button>
    {:else}
      <Button
        variant="confirm"
        disabled={form.busy || !form.ready}
        onclick={() => void form.run(true)}
      >
        件数を確認する
      </Button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .count {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
</style>
