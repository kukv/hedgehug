<script lang="ts">
  import { ApiError, unwrap } from "../../core/api/client";
  import { cliStoreRemove } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Takes a Store out of the config. name is the Store that is asked about; the dialog is open while
  // there is one. The clone is deleted unless the check is off; a Store with commits that were not
  // pushed refuses (409 store.unpushed) and can then be unmounted with its clone kept.
  type Props = { name: string | null; onclose: () => void };
  let { name, onclose }: Props = $props();

  const c = core();

  let deleteClone = $state(true);
  let unpushed = $state(false);
  let failure = $state<string | null>(null);
  let busy = $state(false);

  // A new Store asked about starts the dialog afresh.
  $effect(() => {
    void name;
    deleteClone = true;
    unpushed = false;
    failure = null;
  });

  async function remove(keep: boolean): Promise<void> {
    if (!name || busy) return;
    busy = true;
    failure = null;
    try {
      await unwrap(
        c.api.DELETE("/api/stores/{name}", { params: { path: { name }, query: { keep } } }),
      );
      toast({ message: `${name} のマウントを解除しました` });
      // The Store is unmounted; a failed reload must not read as a failed removal.
      await c.stores.reload().catch(() => undefined);
      onclose();
    } catch (e) {
      unpushed = e instanceof ApiError && e.type === "store.unpushed";
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open={name !== null}
  title="{name ?? ""} のマウントを解除"
  width={560}
  cli={name ? cliStoreRemove(name, !deleteClone) : undefined}
  onclose={() => {
    if (!busy) onclose();
  }}
>
  <p class="confirm">
    {name}
    をこのマシンのマウントから外します。config.yaml から消え、サイドバーにも出なくなります。
  </p>
  <label class="check">
    <input type="checkbox" bind:checked={deleteClone}>
    ローカルのクローンも消す
  </label>
  {#if failure}
    <FormError message={failure} />
  {/if}
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    {#if unpushed}
      <Button disabled={busy} onclick={() => remove(true)}>クローンを残して解除</Button>
    {/if}
    <Button variant="danger" disabled={busy} onclick={() => remove(!deleteClone)}>解除する</Button>
  {/snippet}
</Dialog>

<style>
  .confirm {
    margin: 0 0 var(--space-12);
    line-height: 1.6;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    margin-bottom: var(--space-12);
  }
</style>
