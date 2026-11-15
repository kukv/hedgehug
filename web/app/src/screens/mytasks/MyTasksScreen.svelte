<script lang="ts">
  import {
    applyCandidate,
    type CaretTerm,
    candidateSource,
    caretTerm,
    type FilterCandidate,
    get,
    type MyTasksGroupBy,
    myTaskProjects,
    myTasksGroupChoices,
    myTasksNote,
    myTasksPlaceholder,
    myTasksTitle,
  } from "@hedgehug/core";
  import { FilterInput, LayoutSwitch, PageHead } from "@hedgehug/ui";
  import { onMount, tick } from "svelte";
  import { myTaskBase, myTasks } from "../../myTasks";
  import { createCandidates } from "../../search";
  import SearchHelp from "../search/SearchHelp.svelte";
  import MyTaskGroups from "./MyTaskGroups.svelte";

  // マイタスクの画面(04「マイタスク」、E4a)。絞り込み欄・まとめ方の切り替えと、本体(MyTaskGroups)を出す。
  // 打ち方と候補の選び方は検索の画面(SearchScreen.svelte)と同じ。
  interface Props {
    ondrawer: () => void;
    alert: boolean;
  }

  let { ondrawer, alert }: Props = $props();

  const view = myTasks.state;
  let help = $state(false);

  // 候補は、入力と、候補の元が読み終わったときだけ作り直す。term は、candidates を作った語(選んだときに置き換える範囲)。
  // `project:` の候補は、担当しているタスクのあるプロジェクトだけにする。
  let term = $state<CaretTerm | null>(null);
  let candidates = $state<FilterCandidate[]>([]);
  const sources = createCandidates(
    (source) => {
      if (term && candidateSource(term.key) === source) candidates = sources.list(term);
    },
    () => ({ kind: "myTasks", projects: myTaskProjects(get(myTaskBase.rows) ?? []) }),
  );
  let filter = $state<HTMLElement>();
  // `project:` の候補の範囲は絞り込みなしの行で決まるので、その行が読み終わったら作り直す。
  const baseRows = myTaskBase.rows;
  $effect(() => {
    void $baseRows;
    if (term?.key === "project") candidates = sources.list(term);
  });

  // 空の表示の見分けに、絞り込みなしの行を読む(アプリの開始で読むようになるまで、画面を開くたびに読む)。
  onMount(() => {
    void myTasks.open();
    void myTaskBase.reload();
    return () => sources.stop();
  });

  function pick(text: string, caret: number) {
    term = caretTerm(text, caret);
    candidates = sources.list(term);
  }

  function oninput(text: string, caret: number) {
    myTasks.setFilter(text);
    pick(text, caret);
  }

  async function onchoose(index: number) {
    const c = candidates[index];
    if (!term || !c) return;
    const next = applyCandidate($view.filter, term, c);
    myTasks.setFilter(next.text);
    term = null;
    candidates = [];
    // 欄の値が入れ替わるとカーソルが末尾に移るので、入れたものの直後に戻す。
    await tick();
    filter
      ?.querySelector<HTMLInputElement>("[data-filter-input]")
      ?.setSelectionRange(next.caret, next.caret);
  }
</script>

<PageHead title={myTasksTitle} note={myTasksNote} {ondrawer} {alert} />
<div class="filter" bind:this={filter}>
  <FilterInput
    value={$view.filter}
    placeholder={myTasksPlaceholder}
    options={candidates}
    errors={$view.errors}
    {oninput}
    oncaret={pick}
    onchoose={(i) => void onchoose(i)}
    onhelp={() => (help = true)}
  />
</div>
<div class="toolbar">
  <LayoutSwitch
    label="まとめ方"
    choices={myTasksGroupChoices}
    value={$view.groupBy}
    onchange={(id) => myTasks.setGroupBy(id as MyTasksGroupBy)}
  />
</div>
<MyTaskGroups />
<SearchHelp open={help} onclose={() => (help = false)} />

<style>
  .filter {
    padding: 12px 24px 0;
  }
  .toolbar {
    padding: 10px 24px 6px;
  }
  @container frame (max-width: 860px) {
    .filter,
    .toolbar {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
