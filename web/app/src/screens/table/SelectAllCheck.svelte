<script lang="ts">
  import { headerCheck, type RowSelection, type SearchRow, selectable } from "@hedgehug/core";
  import { RowCheck } from "@hedgehug/ui";

  // 見出しの行の先頭の「すべて選択」(04「選択」、E2a-16)。選べる行(削除済みでない行。同じアイテムは 1 回)が
  // すべて選ばれていればチェック済み。押すと、チェック済みなら選択をすべて外し、そうでなければ選べる行をすべて選ぶ。
  // 選べる行が 0 件なら押せない。
  interface Props {
    rows: SearchRow[];
    selection: RowSelection;
  }

  let { rows, selection }: Props = $props();

  const picked = $derived(selection.rows);
  const checked = $derived(headerCheck(rows, $picked));
  const none = $derived(selectable(rows).length === 0);
</script>

<RowCheck
  label="すべて選択"
  {checked}
  disabled={none}
  onchange={() => (checked ? selection.clear() : selection.set(selectable(rows)))}
/>
