<script lang="ts">
  import { PageHead } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { selection } from "../../palette";
  import { search, searchSelection, selectedOne } from "../../search";
  import SearchBody from "./SearchBody.svelte";
  import SearchFilter from "./SearchFilter.svelte";

  // 検索の画面(04「検索」「画面の構成」、E1-17〜E1-20)。見出しと、絞り込み欄(SearchFilter)と、本体(SearchBody。件数・
  // テーブルかツリー)を出す。
  interface Props {
    ondrawer: () => void;
    alert: boolean;
  }

  let { ondrawer, alert }: Props = $props();

  // パレットの「選んでいるアイテム」は、ちょうど 1 行を選んでいるときの行(E2a-18)。
  $effect(() => selection.provide(2, selectedOne));

  // 選択は画面ごとに持つので、画面を離れたら解除する(04「選択」)。
  onMount(() => {
    void search.open();
    return () => searchSelection.clear();
  });
</script>

<PageHead
  title="検索"
  note="すべての Store のアイテムを、条件で絞り込んで見ます"
  {ondrawer}
  {alert}
/>
<SearchFilter {search} />
<SearchBody {search} />
