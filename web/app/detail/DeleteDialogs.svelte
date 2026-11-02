<script lang="ts">
  import type { ShowView, TaskView } from "../../core/api/client";
  import { cliPurge, cliRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { router } from "../router";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import {
    collectDescendants,
    deleteTask,
    probePurge,
    purgeTargetsOf,
    purgeTask,
    restoreTask,
    type Target,
  } from "./delete-actions";

  let { show, variant }: { show: ShowView; variant: "delete" | "deleted" } = $props();

  const { api, cache } = core();
  const deps = { api, cache };
  const ref = $derived({ ulid: show.ulid, store: show.store });

  let open = $state<"delete" | "purge" | null>(null);
  let descendants = $state<TaskView[] | null>(null);
  let targets = $state<Target[] | null>(null);
  let typed = $state("");
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

  async function openPurge() {
    const mine = ++round;
    open = "purge";
    error = "";
    typed = "";
    targets = null;
    try {
      const found = await probePurge(deps, ref);
      if (mine === round) targets = found;
    } catch (e) {
      // Not a purge.unconfirmed: no confirmation to show, only the error.
      if (mine === round) error = errorMessage(e);
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

  async function confirmPurge() {
    if (!targets || busy) return;
    busy = true;
    error = "";
    // Read before the await: once the task is purged the cache forgets it, and `show` is gone.
    const { id } = show;
    try {
      await purgeTask(
        deps,
        ref,
        targets.map((t) => t.ulid),
      );
      open = null;
      toast({ message: `${id} を完全に削除しました` });
      router.go("/");
    } catch (e) {
      const again = purgeTargetsOf(e);
      if (again) {
        targets = again;
        typed = "";
        error = "対象が変わりました。もう一度確認してください";
      } else {
        error = errorMessage(e);
      }
    } finally {
      busy = false;
    }
  }

  const shown = $derived((descendants ?? []).slice(0, 5));
  const more = $derived(Math.max(0, (descendants ?? []).length - 5));
  const deleteLabel = $derived(
    descendants && descendants.length > 0 ? `${descendants.length + 1} 件を削除` : "削除",
  );
</script>

{#if variant === "delete"}
  <Button size="sm" onclick={() => void openDelete()}>削除</Button>
{:else}
  <Button size="sm" disabled={busy} onclick={() => void restore()}>復元</Button>
  <Button size="sm" variant="danger" onclick={() => void openPurge()}>完全に削除</Button>
{/if}

{#if open === "delete"}
  <Dialog
    open
    title="{show.id} {show.title} を削除"
    width={560}
    cli={cliRm(show.id)}
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
          <li><span class="mono">{t.id}</span> {t.title}</li>
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
{:else if open === "purge"}
  <Dialog open title="{show.id} を完全に削除" width={560} cli={cliPurge(show.id)} onclose={close}>
    <p>タスクのファイルとコメントを消します。Hedgehug からは復元できません。</p>
    <p>git の履歴には残ります。番号 {show.id} は再利用しません。</p>
    {#if targets === null}
      {#if !error}
        <p class="muted">対象を調べています…</p>
      {/if}
    {:else}
      <ul>
        {#each targets as t (t.ulid)}
          <li>
            <span class="mono">{t.id}</span> {t.title} <span class="mono muted">{t.ulid}</span>
          </li>
        {/each}
      </ul>
      <label class="confirm">
        <span
          >履歴を書き換える操作です。確認のため Store 名
          <strong>{show.store}</strong>
          を入力してください</span
        >
        <input type="text" aria-label="Store 名" bind:value={typed} autocomplete="off">
      </label>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#snippet footer()}
      <Button disabled={busy} onclick={close}>キャンセル</Button>
      <Button
        variant="danger"
        disabled={busy || targets === null || targets.length === 0 || typed !== show.store}
        onclick={() => void confirmPurge()}
        >完全に削除</Button
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
  .confirm {
    display: grid;
    gap: var(--space-6);
    font-size: var(--fs-13);
  }
  input {
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font: inherit;
    font-family: var(--font-mono);
    color: var(--ink);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-12);
  }
</style>
