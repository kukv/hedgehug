<script lang="ts">
  import {
    applyCandidate,
    type CaretTerm,
    candidateSource,
    caretTerm,
    type FilterCandidate,
    readOnlyBands,
  } from "@hedgehug/core";
  import { FilterInput, PageHead, ReadOnlyBand } from "@hedgehug/ui";
  import { onMount, tick } from "svelte";
  import { selection } from "../../palette";
  import { createCandidates, search, searchSelection, selectedOne } from "../../search";
  import SearchBody from "./SearchBody.svelte";
  import SearchHelp from "./SearchHelp.svelte";

  // 検索の画面(04「検索」「画面の構成」、E1-17〜E1-20)。絞り込み欄と、本体(SearchBody。件数・テーブルかツリー)を出す。
  interface Props {
    ondrawer: () => void;
    alert: boolean;
  }

  let { ondrawer, alert }: Props = $props();

  const view = search.state;
  const rows = $derived($view.result?.rows ?? []);
  const errors = $derived($view.result?.errors ?? []);
  const bands = $derived(readOnlyBands(rows));
  let help = $state(false);
  // パレットの「選んでいるアイテム」は、ちょうど 1 行を選んでいるときの行(E2a-18)。
  $effect(() => selection.provide(2, selectedOne));

  // 候補は、入力と、候補の元が読み終わったときだけ作り直す(FilterInput は候補の配列が替わると選びを先頭に戻す)。
  // term は、candidates を作った語(選んだときに置き換える範囲)。
  let term = $state<CaretTerm | null>(null);
  let candidates = $state<FilterCandidate[]>([]);
  // 読み終えた元が、いまの語のキーの候補に使うものなら作り直す。
  const sources = createCandidates((source) => {
    if (term && candidateSource(term.key) === source) candidates = sources.list(term);
  });
  let filter = $state<HTMLElement>();

  // 選択は画面ごとに持つので、画面を離れたら解除する(04「選択」)。
  onMount(() => {
    void search.open();
    return () => {
      sources.stop();
      searchSelection.clear();
    };
  });

  // pick は、カーソルの位置の語で候補を作り直す。入力のときも、カーソルだけが動いたときも呼ぶ。
  function pick(text: string, caret: number) {
    term = caretTerm(text, caret);
    candidates = sources.list(term);
  }

  function oninput(text: string, caret: number) {
    search.setQuery(text);
    pick(text, caret);
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
    filter
      ?.querySelector<HTMLInputElement>("[data-filter-input]")
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
<div class="filter" bind:this={filter}>
  <FilterInput
    value={$view.query}
    placeholder="絞り込み 例: assignee:@me status:InProgress system:Web"
    options={candidates}
    {errors}
    {oninput}
    oncaret={pick}
    onchoose={(i) => void onchoose(i)}
    onhelp={() => (help = true)}
  />
</div>
<SearchBody />
<SearchHelp open={help} onclose={() => (help = false)} />

<style>
  .filter {
    padding: 12px 24px 0;
  }
  @container frame (max-width: 860px) {
    .filter {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
