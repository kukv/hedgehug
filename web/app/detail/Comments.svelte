<script lang="ts">
  import { type ShowView, unwrap } from "../../core/api/client";
  import { formatDateTime } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { kbdLabel } from "../keys";
  import { toast } from "../toast.svelte";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import BodyEditor from "./BodyEditor.svelte";

  type CommentView = NonNullable<ShowView["comments"]>[number];

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache } = core();

  // What was written here, until the task is read again (show.updated changes) and has it.
  let overlay = $state<{
    added: CommentView[];
    changed: Record<string, CommentView>;
    removed: string[];
  }>({ added: [], changed: {}, removed: [] });
  $effect(() => {
    void show.updated;
    overlay = { added: [], changed: {}, removed: [] };
  });
  const list = $derived.by(() => {
    const base = show.comments ?? [];
    return [
      ...base
        .filter((c) => !overlay.removed.includes(c.ulid))
        .map((c) => overlay.changed[c.ulid] ?? c),
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

  async function post() {
    const body = draft.trim();
    if (body === "") return;
    formError = "";
    try {
      const r = await unwrap(
        api.POST("/api/tasks/{id}/comments", {
          params: { path: { id: show.ulid }, query: { store: show.store } },
          body: { body },
        }),
      );
      cache.noteOwnCommit(r.commit);
      overlay.added = [...overlay.added, r.comment];
      draft = "";
      writer?.setText("");
    } catch (e) {
      formError = errorMessage(e);
    }
  }

  function startEdit(c: CommentView) {
    editingId = c.ulid;
    editDraft = c.body;
    errors = {};
  }

  async function saveEdit(c: CommentView) {
    errors = {};
    try {
      const r = await unwrap(
        api.PATCH("/api/tasks/{id}/comments/{cid}", {
          params: { path: { id: show.ulid, cid: c.ulid }, query: { store: show.store } },
          body: { body: editDraft },
        }),
      );
      cache.noteOwnCommit(r.commit);
      overlay.changed = { ...overlay.changed, [c.ulid]: r.comment };
      editingId = null;
    } catch (e) {
      errors = { ...errors, [c.ulid]: errorMessage(e) };
    }
  }

  async function remove(c: CommentView) {
    errors = {};
    try {
      const r = await unwrap(
        api.DELETE("/api/tasks/{id}/comments/{cid}", {
          params: { path: { id: show.ulid, cid: c.ulid }, query: { store: show.store } },
        }),
      );
      cache.noteOwnCommit(r.commit);
      overlay.removed = [...overlay.removed, c.ulid];
      toast({ message: "コメントを削除しました" });
    } catch (e) {
      errors = { ...errors, [c.ulid]: errorMessage(e) };
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
              <Button size="sm" variant="primary" onclick={() => void saveEdit(c)}>保存</Button>
              <Button size="sm" onclick={() => (editingId = null)}>キャンセル</Button>
            </div>
          {:else}
            <p class="body">{c.body}</p>
            {#if !readonly}
              <div class="row">
                <Button size="sm" variant="ghost" onclick={() => startEdit(c)}>編集</Button>
                <Button size="sm" variant="ghost" onclick={() => void remove(c)}>削除</Button>
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
      <Button variant="primary" disabled={draft.trim() === ""} onclick={() => void post()}
        >コメント</Button
      >
    </div>
    {#if formError}
      <p class="error" role="alert">{formError}</p>
    {/if}
  {/if}
</section>

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
