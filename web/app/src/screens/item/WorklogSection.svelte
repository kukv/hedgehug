<script lang="ts">
  import {
    type DetailView,
    formatDay,
    readOnlyNote,
    worklogDeleteDialog,
    worklogHeading,
  } from "@hedgehug/core";
  import { Avatar, Button, ConfirmDialog, IconButton, Input } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { syncNow, worklogs } from "../../wire";

  // 作業時間の区画(02「作業時間」)。行に編集はない。入れ間違いは、削除して入れ直す。
  // 時間の欄は数の入力欄で、0.25 時間ずつの増減はブラウザの矢印とキーに任せる。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const store = worklogs.state;
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  const heading = $derived(worklogHeading($store.rows, v.item));
  const year = $derived($syncNow.getFullYear());

  // アイテムが変わったときだけ開き直す(画面への反映で v が新しくなっても、入力は残す)。
  $effect(() => {
    void worklogs.open(v.store.id, v.item.id);
  });
  onDestroy(() => worklogs.close());

  // メモ欄の Enter でも記録する。日本語入力の変換を確定する Enter では記録しない。
  function memoKeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void worklogs.add();
  }
</script>

<section class="peek-sec" aria-label="作業時間">
  <h3>作業時間 <span class="muted">{heading}</span></h3>
  {#each $store.rows as w (w.id)}
    <div class="row">
      <Avatar name={w.byName === "不明なユーザー" ? "" : w.byName} title={w.byName} />
      <span class="hours">{w.hours} 時間</span>
      <span class="memo">{w.memo}</span>
      <span class="date">{formatDay(w.date, year)}</span>
      <IconButton
        label="作業時間を削除"
        size="xs"
        disabled={v.readOnly}
        title={note}
        onclick={() => worklogs.askDelete(w.id)}
        >×</IconButton
      >
    </div>
  {/each}
  <div class="input-row">
    <Input
      label="時間"
      type="number"
      step="0.25"
      min="0"
      placeholder="時間"
      size="sm"
      disabled={v.readOnly}
      title={note}
      bind:value={() => $store.hours, worklogs.setHours}
    />
    <Input
      label="メモ"
      placeholder="メモ(任意)"
      size="sm"
      disabled={v.readOnly}
      title={note}
      onkeydown={memoKeydown}
      bind:value={() => $store.memo, worklogs.setMemo}
    />
    <Button
      size="sm"
      variant="primary"
      disabled={v.readOnly || $store.busy}
      title={note}
      onclick={() => void worklogs.add()}
    >
      記録
    </Button>
  </div>
  {#if $store.error}
    <p class="error" role="alert">{$store.error}</p>
  {/if}
  {#if $store.deleting}
    <ConfirmDialog
      title={worklogDeleteDialog.title}
      lines={$store.deleting.lines}
      ok={worklogDeleteDialog.ok}
      danger
      onok={() => void worklogs.confirmDelete()}
      oncancel={worklogs.cancelDelete}
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
  .muted {
    color: var(--muted);
    font-weight: 400;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
    font-size: 12.5px;
  }
  .hours {
    font-weight: 600;
  }
  .memo {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .date {
    color: var(--muted);
  }
  .input-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .input-row :global(input:first-child) {
    width: 84px;
  }
  .input-row :global(input:nth-child(2)) {
    flex: 1;
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: 12.5px;
  }
</style>
