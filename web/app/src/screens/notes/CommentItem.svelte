<script lang="ts">
  import {
    type Comment,
    type Comments,
    type CommentsState,
    commentAvatar,
    isMemberOf,
    isMine,
    readOnlyNote,
    relativeTime,
    tokenize,
  } from "@hedgehug/core";
  import { Avatar, Button, Markdown, Textarea } from "@hedgehug/ui";
  import { syncNow } from "../../wire";

  // コメント 1 件(02「コメント」)。自分が書いたコメントにだけ、編集と削除のボタンを出す。
  interface Props {
    c: Comment;
    comments: Comments;
    s: CommentsState;
    readOnly: boolean;
  }

  let { c, comments, s, readOnly }: Props = $props();

  const note = $derived(readOnly ? readOnlyNote : undefined);
  const editing = $derived(s.editing?.id === c.id ? s.editing : null);
  const blocks = $derived(tokenize(c.body, isMemberOf(s.members)));
  let area: ReturnType<typeof Textarea> | undefined = $state();

  async function onkeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void comments.saveEdit();
      return;
    }
    // 変えていないときの Esc は、パネルを閉じる Esc に任せる。変えているときは確認を出し、その Esc でパネルは閉じない。
    if (event.key !== "Escape" || editing === null || editing.body === c.body) return;
    event.stopPropagation();
    await comments.escapeEdit();
    area?.focus();
  }
</script>

<article class="comment">
  <header class="meta">
    <Avatar name={commentAvatar(c)} title={c.byName} />
    <span class="who">{c.byName}</span>
    <time class="when" datetime={c.at}>{relativeTime(new Date(c.at), $syncNow)}</time>
    {#if c.edited}
      <span class="when">(編集済み)</span>
    {/if}
    {#if isMine(c, s.me) && !editing}
      <span class="grow"></span>
      <Button size="xs" disabled={readOnly} title={note} onclick={() => comments.startEdit(c.id)}>
        編集
      </Button>
      <Button size="xs" disabled={readOnly} title={note} onclick={() => comments.askDelete(c.id)}>
        削除
      </Button>
    {/if}
  </header>
  {#if editing}
    <Textarea
      bind:this={area}
      label="コメントを編集"
      disabled={readOnly}
      title={note}
      focusOnMount
      {onkeydown}
      bind:value={() => editing.body, comments.setEditing}
    />
    <div class="actions">
      <Button size="sm" onclick={comments.cancelEdit}>キャンセル</Button>
      <Button
        size="sm"
        variant="primary"
        disabled={readOnly || s.busy}
        title={note}
        onclick={() => void comments.saveEdit()}
      >
        保存
      </Button>
    </div>
  {:else}
    <Markdown {blocks} />
  {/if}
</article>

<style>
  .comment {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
  }
  .who {
    font-weight: 600;
  }
  .when {
    color: var(--muted);
  }
  .grow {
    flex: 1;
  }
  .actions {
    display: flex;
    gap: 6px;
    justify-content: flex-end;
  }
</style>
