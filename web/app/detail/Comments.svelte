<script lang="ts">
  import { untrack } from "svelte";
  import type { ShowView } from "../../core/api/client";
  import { cliComment } from "../../core/cli";
  import { formatDateTime } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { kbdLabel } from "../keys";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import BodyEditor from "./BodyEditor.svelte";

  type CommentView = NonNullable<ShowView["comments"]>[number];

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache, accounts } = core();

  // Only the author may edit or delete a comment (the server refuses the others). The accounts are
  // read from GitHub, so they are asked for here; until they are read (or when they cannot be) nobody
  // is offered the buttons. An account ID is compared without case, as config.SameAccount does.
  void accounts.load();
  const mine = useSubscribe(accounts, () =>
    accounts.loaded() && accounts.error() === null
      ? new Set(accounts.accounts().map((a) => a.id.toLowerCase()))
      : new Set<string>(),
  );
  const isMine = (c: CommentView) => c.author !== "" && mine.current.has(c.author.toLowerCase());

  // What was written here, each until the task is read again and says it (or says something newer). from is the
  // text the comment had in the task when it was edited.
  let overlay = $state<{
    added: CommentView[];
    changed: Record<string, { view: CommentView; from: string | undefined }>;
    removed: string[];
  }>({ added: [], changed: {}, removed: [] });
  // A comment commit leaves the task's `updated` as it is, so the comments themselves are the key.
  const readKey = $derived(JSON.stringify(show.comments ?? []));
  $effect(() => {
    void readKey;
    untrack(reconcile);
  });

  // A read that brings the change of someone else, before it has what was written here, keeps what was written.
  function reconcile() {
    const base = show.comments ?? [];
    const has = new Set(base.map((c) => c.ulid));
    const added = overlay.added.filter((a) => !has.has(a.ulid));
    const changed: typeof overlay.changed = {};
    for (const [u, o] of Object.entries(overlay.changed)) {
      const now = base.find((c) => c.ulid === u);
      // A comment posted here that the read does not have yet keeps its edit for the read that has it.
      if (now ? now.body === o.from : added.some((a) => a.ulid === u)) changed[u] = o;
    }
    overlay = {
      added,
      changed,
      removed: overlay.removed.filter((u) => has.has(u)),
    };
  }
  const list = $derived.by(() => {
    const base = show.comments ?? [];
    return [
      ...base
        .filter((c) => !overlay.removed.includes(c.ulid))
        .map((c) => overlay.changed[c.ulid]?.view ?? c),
      ...overlay.added.filter((a) => !base.some((c) => c.ulid === a.ulid)),
    ];
  });

  let draft = $state("");
  let formError = $state("");
  let writer = $state<{ setText(t: string): void } | undefined>();
  let editingId = $state<string | null>(null);
  let editDraft = $state("");
  let errors = $state<Record<string, string>>({});
  const submitKey = kbdLabel("mod+enter");

  // One write at a time: a second key or click before the answer would post the comment twice.
  let writing = $state(false);

  async function post() {
    const body = draft.trim();
    if (body === "" || writing) return;
    writing = true;
    formError = "";
    try {
      const r = await api.addComment(show.ulid, { body }, { store: show.store });
      cache.noteOwnCommit(r.commit);
      overlay.added = [...overlay.added, r.comment];
      draft = "";
      writer?.setText("");
    } catch (e) {
      formError = errorMessage(e);
    } finally {
      writing = false;
    }
  }

  function startEdit(c: CommentView) {
    editingId = c.ulid;
    editDraft = c.body;
    errors = {};
  }

  async function saveEdit(c: CommentView) {
    if (writing) return;
    writing = true;
    errors = {};
    try {
      const r = await api.editComment(
        show.ulid,
        c.ulid,
        { body: editDraft },
        { store: show.store },
      );
      cache.noteOwnCommit(r.commit);
      // A comment posted here and not read yet is edited where it is held too; once a read has it as
      // it was posted, the edit stays over it until the read has the edit.
      const posted = overlay.added.find((a) => a.ulid === c.ulid);
      if (posted) overlay.added = overlay.added.map((a) => (a === posted ? r.comment : a));
      overlay.changed = {
        ...overlay.changed,
        [c.ulid]: {
          view: r.comment,
          from:
            show.comments?.find((x) => x.ulid === c.ulid)?.body ??
            overlay.changed[c.ulid]?.from ??
            posted?.body,
        },
      };
      editingId = null;
    } catch (e) {
      errors = { ...errors, [c.ulid]: errorMessage(e) };
    } finally {
      writing = false;
    }
  }

  // The comment waiting for the confirmation of its deletion.
  let deleting = $state<CommentView | null>(null);
  let deleteError = $state("");
  let deleteBusy = $state(false);

  function askRemove(c: CommentView) {
    deleting = c;
    deleteError = "";
  }

  async function remove(c: CommentView) {
    if (deleteBusy) return;
    deleteBusy = true;
    deleteError = "";
    try {
      const r = await api.removeComment(show.ulid, c.ulid, { store: show.store });
      cache.noteOwnCommit(r.commit);
      if (overlay.added.some((a) => a.ulid === c.ulid)) {
        // posted here and not read yet: what was written here is all there is of it
        const { [c.ulid]: _, ...changed } = overlay.changed;
        overlay.added = overlay.added.filter((a) => a.ulid !== c.ulid);
        overlay.changed = changed;
      }
      // a read sent before the deletion may still bring it
      overlay.removed = [...overlay.removed, c.ulid];
      deleting = null;
      toast({ message: "コメントを削除しました" });
    } catch (e) {
      deleteError = errorMessage(e);
    } finally {
      deleteBusy = false;
    }
  }
</script>

<section aria-labelledby="cm-h">
  <h2 id="cm-h">コメント <span class="count">{list.length}</span></h2>

  <ul>
    {#each list as c (c.ulid)}
      <li>
        <Avatar name={c.author} />
        <div class="main">
          <p class="meta">
            <strong>{c.author}</strong>
            ·
            <span class="mono">{formatDateTime(c.created, new Date())}</span>
          </p>
          {#if editingId === c.ulid}
            <BodyEditor
              value={editDraft}
              store={show.store}
              label="コメントを編集"
              minHeight="4em"
              onedit={(t) => (editDraft = t)}
              onsubmit={() => void saveEdit(c)}
            />
            <div class="row">
              <Button
                size="sm"
                variant="primary"
                disabled={writing}
                onclick={() => void saveEdit(c)}
                >保存</Button
              >
              <Button size="sm" onclick={() => (editingId = null)}>キャンセル</Button>
            </div>
          {:else}
            <p class="body">{c.body}</p>
            {#if !readonly && isMine(c)}
              <div class="row">
                <Button size="sm" variant="ghost" onclick={() => startEdit(c)}>編集</Button>
                <Button size="sm" variant="ghost" onclick={() => askRemove(c)}>削除</Button>
              </div>
            {/if}
          {/if}
          {#if errors[c.ulid]}
            <p class="error" role="alert">{errors[c.ulid]}</p>
          {/if}
        </div>
      </li>
    {/each}
  </ul>

  {#if !readonly}
    <div class="form">
      <BodyEditor
        bind:this={writer}
        value={draft}
        store={show.store}
        label="コメントを書く"
        placeholder="コメントを書く（Markdown · @ で人、# でタスク · {submitKey} で送信）"
        minHeight="4em"
        onedit={(t) => (draft = t)}
        onsubmit={() => void post()}
      />
      <Button
        variant="primary"
        disabled={draft.trim() === "" || writing}
        onclick={() => void post()}
        >コメント</Button
      >
    </div>
    {#if formError}
      <p class="error" role="alert">{formError}</p>
    {/if}
  {/if}
</section>

{#if deleting}
  {@const target = deleting}
  <Dialog
    open
    title="コメントを削除"
    width={560}
    cli={cliComment(show.id, "rm", target.ulid)}
    onclose={() => {
      if (!deleteBusy) deleting = null;
    }}
  >
    <p>このコメントを削除します。元に戻せません。</p>
    {#if deleteError}
      <p class="error" role="alert">{deleteError}</p>
    {/if}
    {#snippet footer()}
      <Button disabled={deleteBusy} onclick={() => (deleting = null)}>キャンセル</Button>
      <Button variant="danger" disabled={deleteBusy} onclick={() => void remove(target)}
        >削除</Button
      >
    {/snippet}
  </Dialog>
{/if}

<style>
  h2 {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .count {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  ul {
    display: grid;
    gap: var(--space-12);
    margin: 0 0 var(--space-12);
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-10);
  }
  .main {
    flex: 1;
    min-width: 0;
  }
  .meta {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .meta strong {
    color: var(--ink);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .body {
    margin: var(--space-4) 0;
    font-size: var(--fs-13);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .row {
    display: flex;
    gap: var(--space-4);
    margin-top: var(--space-4);
  }
  .form {
    display: grid;
    gap: var(--space-8);
    justify-items: end;
  }
  .form > :global(.editor) {
    justify-self: stretch;
  }
  .error {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
