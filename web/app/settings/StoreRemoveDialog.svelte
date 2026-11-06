<script lang="ts">
  import { ApiError } from "../../core/api/client";
  import { cliStoreRemove } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Takes a Store out of the config. name is the Store that is asked about; the dialog is open while
  // there is one. The clone is deleted unless the check is off; a Store with commits that were not
  // pushed refuses (409 store.unpushed) and can then be unmounted with its clone kept. A Store with
  // no remote has nowhere its tasks are pushed to, so the server never deletes its clone: the check
  // starts off for it.
  type Props = { name: string | null; onclose: () => void };
  let { name, onclose }: Props = $props();

  const c = core();

  const NO_REMOTE =
    "remote のない Store は、クローンを消すとタスクが失われるため、クローンごとは解除できません。「クローンを残して解除」を使ってください";

  let deleteClone = $state(true);
  let unpushed = $state(false);
  let failure = $state<string | null>(null);
  let busy = $state(false);

  // A new Store asked about starts the dialog afresh.
  $effect(() => {
    void name;
    deleteClone = !noRemote();
    unpushed = false;
    failure = null;
  });

  // The Store asked about has no remote (a Store the list does not know is taken to have one).
  function noRemote(): boolean {
    const s = c.stores.stores().find((x) => x.name === name);
    return !!s && (s.remotes ?? []).length === 0;
  }

  async function remove(keep: boolean): Promise<void> {
    if (!name || busy) return;
    busy = true;
    failure = null;
    try {
      await c.api.removeStore(name, { keep });
      toast({ message: `${name} のマウントを解除しました` });
      // The Store is unmounted; a failed reload must not read as a failed removal.
      await c.stores.reload().catch(() => undefined);
      onclose();
    } catch (e) {
      unpushed = e instanceof ApiError && e.type === "store.unpushed";
      failure = unpushed && noRemote() ? NO_REMOTE : errorMessage(e);
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
  {#if deleteClone && noRemote()}
    <p class="note">{NO_REMOTE}</p>
  {/if}
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
  .note {
    margin: 0 0 var(--space-12);
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    margin-bottom: var(--space-12);
  }
</style>
