<script lang="ts">
  import {
    appendMention,
    commentDeleteDialog,
    commentPlaceholder,
    mentionTargets,
    type NoteTarget,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, Textarea } from "@hedgehug/ui";
  import { onDestroy, untrack } from "svelte";
  import { itemComments, projectComments } from "../../wire";
  import CommentItem from "./CommentItem.svelte";

  // コメントの区画(02「コメント」「メンション」)。アイテムの詳細パネルとプロジェクトの概要の両方が使う。
  // target.kind は、この区画を作ったあとで変えない(詳細パネルは item、概要は project)。
  interface Props {
    target: NoteTarget;
    readonly: boolean;
  }

  let { target, readonly: readOnly }: Props = $props();

  const comments = untrack(() => (target.kind === "item" ? itemComments : projectComments));
  const store = comments.state;
  const note = $derived(readOnly ? readOnlyNote : undefined);
  let area: ReturnType<typeof Textarea> | undefined = $state();

  // 対象そのもの(Store・種類・ID)が変わったときだけ開き直す(画面への反映で target が新しくなっても、入力は残す)。
  // 対象が変わると、前の入力と一覧は捨てられ、前の対象の読み込みの結果は無視される。
  const identity = $derived(`${target.storeId}/${target.kind}/${target.id}`);
  $effect(() => {
    void identity;
    void comments.open(untrack(() => target));
  });
  onDestroy(() => comments.close());

  async function onkeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void comments.post();
      return;
    }
    // 書きかけがあるときだけ確認する。確認の Esc がパネルを閉じる Esc と重ならないよう止める。ないときの Esc はパネルを閉じる。
    if (event.key !== "Escape" || $store.newBody.trim() === "") return;
    event.stopPropagation();
    await comments.escapeNew();
    area?.focus();
  }
</script>

<section class="peek-sec" aria-label="コメント">
  <h3>コメント</h3>
  {#each $store.rows as c (c.id)}
    <CommentItem {c} {comments} s={$store} {readOnly} />
  {/each}
  <Textarea
    bind:this={area}
    label="コメント"
    placeholder={commentPlaceholder}
    disabled={readOnly}
    title={note}
    {onkeydown}
    bind:value={() => $store.newBody, comments.setNew}
  />
  <div class="row">
    {#each mentionTargets($store.members, $store.me) as login (login)}
      <Button
        size="xs"
        disabled={readOnly}
        title={note}
        onclick={() => comments.setNew(appendMention($store.newBody, login))}
      >
        @{login}
      </Button>
    {/each}
    <span class="grow"></span>
    <Button
      variant="primary"
      size="sm"
      disabled={readOnly || $store.busy}
      title={note}
      onclick={() => void comments.post()}
    >
      送信
    </Button>
  </div>
  {#if $store.deleting}
    <ConfirmDialog
      title={commentDeleteDialog.title}
      lines={commentDeleteDialog.lines}
      ok={commentDeleteDialog.ok}
      danger
      onok={() => void comments.confirmDelete()}
      oncancel={comments.cancelDelete}
    />
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .grow {
    flex: 1;
  }
</style>
