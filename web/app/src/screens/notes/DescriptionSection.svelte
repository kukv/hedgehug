<script lang="ts">
  import { createDescriptionEditor, type NoteTarget, readOnlyNote } from "@hedgehug/core";
  import { Button, Markdown, Textarea } from "@hedgehug/ui";
  import { drafts } from "../../drafts";
  import { listMembers, notes, toasts } from "../../wire";

  // 説明の区画(02「説明」)。アイテムの詳細パネルとプロジェクトの概要の両方で使う。状態と規則は core の
  // createDescriptionEditor が持ち、ここは結び付けと見た目だけ。
  interface Props {
    target: NoteTarget;
    readonly: boolean;
  }

  let { target, readonly }: Props = $props();

  const editor = createDescriptionEditor({
    port: notes,
    toasts,
    target: () => target,
    members: listMembers,
  });
  const view = editor.state;
  const note = $derived(readonly ? readOnlyNote : undefined);
  let area: ReturnType<typeof Textarea> | undefined = $state();

  // 開いたとき、画面への反映で target が新しくなるたびに、保存してある説明を読み直す(編集中の入力欄は書き換えない)。
  $effect(() => {
    void target;
    void editor.load();
  });

  // 書きかけとして登録する(確認と、反映を待つ条件)。key の "description:" で確認の本文が決まる。
  $effect(() =>
    drafts.register(`description:${target.kind}/${target.id}`, editor.dirty, editor.held),
  );

  async function onkeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void editor.save();
      return;
    }
    // 入力欄の中の Esc は、違う入力があるときだけ確認する。確認の Esc がパネルを閉じる Esc と重ならないよう止める。
    if (event.key !== "Escape" || !editor.dirty()) return;
    event.stopPropagation();
    if (await drafts.confirmLeave(`description:${target.kind}/${target.id}`)) editor.cancel();
    else area?.focus();
  }
</script>

<section class="peek-sec" aria-label="説明">
  <h3>説明</h3>
  {#if $view.loaded}
    {#if $view.editing}
      <Textarea
        bind:this={area}
        label="説明"
        rows={8}
        focusOnMount
        {onkeydown}
        bind:value={() => $view.input, editor.setInput}
      />
      <p class="hint">Markdown を使用できます。@ログイン名 でメンション</p>
      <div class="actions">
        <Button variant="primary" disabled={$view.saving} onclick={() => void editor.save()}>
          保存
        </Button>
        <Button onclick={editor.cancel}>キャンセル</Button>
      </div>
    {:else if $view.text === ""}
      <button type="button" class="add" disabled={readonly} title={note} onclick={editor.start}>
        説明を追加
      </button>
    {:else}
      <!-- 説明の部分を押すと入力欄になる。押す場所は、内容の下に敷いた本物のボタン(中のリンクは押せる)。 -->
      <div class="view">
        <button
          type="button"
          class="hit"
          aria-label="説明を編集"
          disabled={readonly}
          title={note}
          onclick={editor.start}
        ></button>
        <div class="content"><Markdown blocks={$view.blocks} /></div>
      </div>
    {/if}
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
  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 12.5px;
  }
  .actions {
    display: flex;
    gap: 6px;
  }
  .add {
    align-self: flex-start;
    border: 0;
    background: none;
    padding: 2px 0;
    color: var(--muted);
  }
  .add:hover:not(:disabled) {
    color: var(--ink);
    text-decoration: underline;
  }
  .add:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }
  .view {
    position: relative;
    border-radius: 6px;
  }
  .hit {
    position: absolute;
    inset: -4px -6px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    cursor: text;
  }
  .hit:hover:not(:disabled) {
    background: var(--hover);
  }
  .hit:disabled {
    cursor: default;
  }
  .content {
    position: relative;
    pointer-events: none;
  }
  .content :global(a) {
    pointer-events: auto;
  }
</style>
