<script lang="ts">
  import { type ProjectView, readOnlyNote } from "@hedgehug/core";
  import { Button, Input } from "@hedgehug/ui";
  import { projectEditor } from "../../projectEditor";
  import { projectDelete } from "../../wireActions";

  // プロジェクトの設定タブ(02「画面の見出しと構成」)。A4b は「名前」「削除」の 2 区画だけ。「使うステータス」
  // 「ワークフローの制約」「テンプレート」「自動化」と見出しの下の 1 行は B5、「別のワークスペースへ移動」は A5 が足す。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const disabled = $derived(view.store.readOnly);
  const hint = $derived(disabled ? readOnlyNote : undefined);
  const note = projectDelete.note;

  // 名前の入力欄。概要のタイトルと同じ保存(フォーカスを外す・Enter)と断り方で、フォーカスしているあいだは、
  // 画面への反映(保存して読み直したもの)で書き換えない(06)。editing は $state にしない(OverviewProps と同じ理由)。
  let title = $state("");
  let editing = false;
  $effect(() => {
    const saved = p.title;
    if (!editing) title = saved;
  });

  // 保存できなかった(空・断り)ときは、保存済みの名前に戻す。
  async function save() {
    editing = false;
    if (!(await projectEditor.rename(p, title))) title = p.title;
  }

  // Enter でもフォーカスを外して保存する。日本語の変換を確定する Enter は除く。
  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }

  // 「削除」の区画の説明の件数と保持日数は、サーバーの予告から読む。アイテムの数が変わったときも読み直す。
  $effect(() => {
    void view.items.length;
    void projectDelete.load(p);
  });
</script>

<div class="settings">
  <section class="sec">
    <h2>名前</h2>
    <Input
      label="名前"
      size="sm"
      {disabled}
      title={hint}
      bind:value={title}
      onfocus={() => (editing = true)}
      onblur={() => void save()}
      {onkeydown}
    />
  </section>
  <section class="sec danger">
    <h2>削除</h2>
    {#if $note?.projectId === p.id}
      <p class="text">{$note.text}</p>
    {/if}
    <div>
      <Button variant="danger" {disabled} title={hint} onclick={() => void projectDelete.open(p)}
        >プロジェクトを削除</Button
      >
    </div>
  </section>
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 640px;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h2 {
    font-size: 14px;
    margin: 0;
  }
  .text {
    margin: 0;
  }
  .danger {
    padding: 12px 16px;
    border: 1px solid color-mix(in srgb, var(--danger) 40%, var(--line));
    border-radius: 8px;
  }
</style>
