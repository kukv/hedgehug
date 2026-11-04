<script lang="ts">
  import type { ShowView, TaskView } from "../../core/api/client";
  import { cliRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { readOnlySentence } from "../read-only";
  import { useDisplayId } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import { collectDescendants, deleteTask, restoreTask } from "./delete-actions";

  // readOnly is why the Store cannot be written (in Japanese), or null: the buttons are off then.
  let {
    show,
    variant,
    readOnly = null,
  }: { show: ShowView; variant: "delete" | "deleted"; readOnly?: string | null } = $props();

  const { api, cache, stores } = core();
  const did = useDisplayId(stores);
  const deps = { api, cache };
  const ref = $derived({ ulid: show.ulid, store: show.store });

  let open = $state<"delete" | null>(null);
  let descendants = $state<TaskView[] | null>(null);
  let busy = $state(false);
  let error = $state("");

  // The number of the open: the answer of an earlier one does not fill a later one.
  let round = 0;

  // Closing while a write runs would leave its error in a dialog nobody sees.
  function close() {
    if (!busy) open = null;
  }

  async function openDelete() {
    const mine = ++round;
    open = "delete";
    error = "";
    descendants = null;
    try {
      const found = await collectDescendants(api, show.store, show.ulid);
      if (mine === round) descendants = found;
    } catch (e) {
      if (mine !== round) return;
      error = errorMessage(e);
      descendants = [];
    }
  }

  async function confirmDelete() {
    if (busy) return;
    busy = true;
    error = "";
    // Read before the await: the answer of the write changes the cache, and `show` with it.
    const { title } = show;
    const target = ref;
    try {
      await deleteTask(deps, target);
      open = null;
      toast({
        message: `「${title}」を削除しました`,
        undo: async () => {
          await restoreTask(deps, target);
        },
      });
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function restore() {
    if (busy) return;
    busy = true;
    const { id } = show;
    try {
      await restoreTask(deps, ref);
      toast({ message: `${id} を復元しました` });
    } catch (e) {
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }

  const off = $derived(readOnly !== null);
  const offTitle = $derived(readOnly !== null ? readOnlySentence(readOnly) : undefined);
  const shown = $derived((descendants ?? []).slice(0, 5));
  const more = $derived(Math.max(0, (descendants ?? []).length - 5));
  const deleteLabel = $derived(
    descendants && descendants.length > 0 ? `${descendants.length + 1} 件を削除` : "削除",
  );
</script>

{#if variant === "delete"}
  <Button size="sm" disabled={off} why={offTitle} onclick={() => void openDelete()}>削除</Button>
{:else}
  <Button size="sm" disabled={busy || off} why={offTitle} onclick={() => void restore()}
    >復元</Button
  >
{/if}

{#if open === "delete"}
  <Dialog
    open
    title="{did.current(show.store, show.id)} {show.title} を削除"
    width={560}
    cli={cliRm(did.current(show.store, show.id))}
    onclose={close}
  >
    {#if descendants === null}
      <p class="muted">配下を調べています…</p>
    {:else if descendants.length > 0}
      <p>
        配下の {descendants.length} 件も一緒に削除します。ファイルは残るので、あとから復元できます
      </p>
      <ul>
        {#each shown as t (t.ulid)}
          <li><span class="mono">{did.current(show.store, t.id)}</span> {t.title}</li>
        {/each}
        {#if more > 0}
          <li class="muted">ほか {more} 件</li>
        {/if}
      </ul>
    {:else}
      <p>ファイルは残るので、あとから復元できます。</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#snippet footer()}
      <Button disabled={busy} onclick={close}>キャンセル</Button>
      <Button
        variant="danger"
        disabled={busy || descendants === null}
        onclick={() => void confirmDelete()}
        >{deleteLabel}</Button
      >
    {/snippet}
  </Dialog>
{/if}

<style>
  p {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  ul {
    margin: 0 0 var(--space-12);
    padding: 0;
    list-style: none;
    font-size: var(--fs-13);
  }
  li {
    padding: var(--space-4) 0;
    overflow-wrap: anywhere;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .muted {
    color: var(--muted);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-12);
  }
</style>
