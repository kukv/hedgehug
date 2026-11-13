<script lang="ts">
  import {
    type DetailView,
    type LinkKind,
    markerCopiedMessage,
    markerCopyFailedMessage,
    markerOf,
    noAccountNote,
    noLinksText,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { linkDialog, linkList } from "../../github";
  import { toasts } from "../../wire";
  import LinkDialog from "./LinkDialog.svelte";
  import LinkRow from "./LinkRow.svelte";

  // 詳細パネルの GitHub の欄(02「GitHub」、07「PR と Issue のリンク」)。開いたら一覧を出し、GitHub から取り直す。
  // 閲覧のみの Store では、リンクの追加と解除を無効にする(印のコピーと外部リンクは使える)。アカウントが
  // 1 つもなければ、リンクの追加を無効にする(07「GitHub を使わない場合」)。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();
  const model = linkList.state;
  const storeId = $derived(v.store.id);
  const itemId = $derived(v.item.id);
  const addNote = $derived(
    v.readOnly ? readOnlyNote : $model.hasAccount ? undefined : noAccountNote,
  );
  const removeNote = $derived(v.readOnly ? readOnlyNote : undefined);

  $effect(() => {
    void linkList.load(storeId, itemId);
  });
  onDestroy(() => linkList.reset());
  // リンク直後の読み直しで届くプロジェクトのリポジトリを、開いているダイアログにも渡す。
  $effect(() => linkDialog.setRepos(v.project.repos));

  function open(kind: LinkKind) {
    linkDialog.open(kind, storeId, itemId, v.project.repos);
  }

  async function copyMarker() {
    try {
      await navigator.clipboard.writeText(markerOf(itemId));
      toasts.show(markerCopiedMessage);
    } catch {
      toasts.show(markerCopyFailedMessage, { error: true });
    }
  }
</script>

<section class="peek-sec" aria-label="GitHub">
  <div class="sec-head">
    <h3>GitHub</h3>
    <span class="grow"></span>
    <Button size="sm" onclick={() => void copyMarker()}>PR 用の印をコピー</Button>
  </div>
  {#if $model.loaded && $model.rows.length === 0}
    <p class="muted">{noLinksText}</p>
  {/if}
  {#each $model.rows as row (`${row.kind}:${row.repo}#${row.number}`)}
    <LinkRow
      {row}
      disabled={v.readOnly}
      title={removeNote}
      onremove={() => void linkList.remove(row)}
    />
  {/each}
  <div class="add-row">
    <Button size="sm" disabled={addNote !== undefined} title={addNote} onclick={() => open("pr")}
      >PR をリンク</Button
    >
    <Button size="sm" disabled={addNote !== undefined} title={addNote} onclick={() => open("issue")}
      >Issue をリンク</Button
    >
  </div>
  <LinkDialog />
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .add-row {
    display: flex;
    gap: 6px;
  }
</style>
