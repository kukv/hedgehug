<script lang="ts">
  import {
    applyCandidate,
    type CaretTerm,
    caretTerm,
    emptyText,
    type FilterCandidate,
    readOnlyBands,
  } from "@hedgehug/core";
  import { Button, FilterInput, PageHead, ReadOnlyBand } from "@hedgehug/ui";
  import { onMount, tick } from "svelte";
  import { createCandidates, search } from "../../search";
  import { itemDialog } from "../../wire";
  import SearchHelp from "./SearchHelp.svelte";
  import SearchTable from "./SearchTable.svelte";

  // 検索の画面(04「検索」「画面の構成」、E1-17〜E1-20)。絞り込み欄・件数・テーブルを出す。
  interface Props {
    ondrawer: () => void;
    alert: boolean;
  }

  let { ondrawer, alert }: Props = $props();

  const view = search.state;
  const rows = $derived($view.result?.rows ?? []);
  const errors = $derived($view.result?.errors ?? []);
  const bands = $derived(readOnlyBands(rows));
  // 最初の検索が終わるまでは、空の表示を出さない(開いた直後に「まだ何もありません」がちらつかない)。
  const empty = $derived($view.result ? emptyText($view.query, rows.length, errors) : null);
  let help = $state(false);

  // 候補は、入力と、候補の元が読み終わったときだけ作り直す(FilterInput は候補の配列が替わると選びを先頭に戻す)。
  // term は、candidates を作った語(選んだときに置き換える範囲)。
  let term = $state<CaretTerm | null>(null);
  let candidates = $state<FilterCandidate[]>([]);
  const sources = createCandidates(() => {
    if (term) candidates = sources.list(term);
  });

  onMount(() => void search.open());

  function oninput(text: string, caret: number) {
    search.setQuery(text);
    term = caretTerm(text, caret);
    candidates = sources.list(term);
  }

  async function onchoose(index: number) {
    const c = candidates[index];
    if (!term || !c) return;
    const next = applyCandidate($view.query, term, c);
    search.setQuery(next.text);
    term = null;
    candidates = [];
    // 欄の値が入れ替わるとカーソルが末尾に移るので、入れたものの直後に戻す。
    await tick();
    document
      .querySelector<HTMLInputElement>("[data-filter-input]")
      ?.setSelectionRange(next.caret, next.caret);
  }
</script>

<PageHead
  title="検索"
  note="すべての Store のアイテムを、条件で絞り込んで見ます"
  {ondrawer}
  {alert}
/>
{#each bands as band (band)}
  <ReadOnlyBand text={band} />
{/each}
<div class="filter">
  <FilterInput
    value={$view.query}
    placeholder="絞り込み 例: assignee:@me status:InProgress system:Web"
    options={candidates}
    {errors}
    {oninput}
    onchoose={(i) => void onchoose(i)}
    onhelp={() => (help = true)}
  />
</div>
<div class="toolbar">
  <span class="count">{rows.length} 件</span>
</div>
<div class="content">
  {#if empty}
    <div class="empty">
      <p class="empty-title">{empty.title}</p>
      {#if empty.body}
        <p class="empty-note">{empty.body}</p>
      {/if}
      {#if empty.create}
        <div class="create">
          <Button variant="primary" size="sm" onclick={() => void itemDialog.open()}>作成</Button>
        </div>
      {/if}
    </div>
  {:else if rows.length > 0}
    <SearchTable {rows} />
  {/if}
</div>
<SearchHelp open={help} onclose={() => (help = false)} />

<style>
  .filter {
    padding: 12px 24px 0;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 24px;
  }
  .count {
    color: var(--muted);
    font-size: 12.5px;
    font-variant-numeric: tabular-nums;
  }
  .content {
    flex: 1;
    overflow: auto;
    padding: 0 24px 40px;
    min-height: 0;
  }
  .empty {
    padding: 40px 0;
    text-align: center;
  }
  .empty-title {
    margin: 0;
    font-weight: 500;
  }
  .empty-note {
    margin: 6px auto 12px;
    max-width: 520px;
    color: var(--muted);
    font-size: 12.5px;
  }
  .create {
    margin-top: 12px;
  }
  @container frame (max-width: 860px) {
    .filter,
    .toolbar,
    .content {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
