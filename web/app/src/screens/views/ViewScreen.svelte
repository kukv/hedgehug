<script lang="ts">
  import { createViewScreen, writable } from "@hedgehug/core";
  import { Button, IconButton, PageHead } from "@hedgehug/ui";
  import { onMount, untrack } from "svelte";
  import { selection } from "../../palette";
  import { activate, searchSelection, selectedOne } from "../../search";
  import { createViewSearch, views } from "../../views";
  import { toasts } from "../../wireBase";
  import SearchBody from "../search/SearchBody.svelte";
  import SearchFilter from "../search/SearchFilter.svelte";

  // 保存したビューの画面(04「保存したビュー」、E3b-12)。見出しの上に「保存したビュー」、見出しはビューの名前、右に
  // 未保存の変更があるときだけ印とボタン「変更を破棄」「保存」、その右にメニューのボタン。中身は検索の画面と同じ部品で、
  // 画面の内容は画面の状態の鍵 view:{id} に置く(E3b-3)。画面は id ごとに作り直す(shell/Screen.svelte の key)。
  interface Props {
    id: string;
    ondrawer: () => void;
    alert: boolean;
  }

  let { id, ondrawer, alert }: Props = $props();

  // 画面は id ごとに作り直すので、流れは最初の id で作る。
  const viewId = untrack(() => id);
  const search = createViewSearch(viewId);
  // ready は、最初の open が画面の状態を読み終えたか(読む前は印を出さない。E3b-4)。
  const ready = writable(false);
  const screen = createViewScreen({ id: viewId, views, search, ready, toasts });
  const dirty = screen.dirty;

  const list = views.list;
  const view = $derived($list.find((v) => v.id === id));
  const name = $derived(view?.name ?? "");

  // パレットの「選んでいるアイテム」は、検索の画面と同じ(E3b-14)。
  $effect(() => selection.provide(2, selectedOne));

  // 開いているあいだは、Store が変わった知らせと selectedOne がこの流れを使う。離れたら戻し、選択を解除する。
  onMount(() => {
    const restore = activate(search);
    void search.open().then(() => ready.set(true));
    return () => {
      restore();
      searchSelection.clear();
    };
  });
</script>

<PageHead title={name} {ondrawer} {alert}>
  {#snippet crumbs()}
    <span>保存したビュー</span>
  {/snippet}
  {#snippet actions()}
    {#if $dirty}
      <span class="unsaved">未保存の変更</span>
      <Button size="sm" onclick={() => screen.discard()}>変更を破棄</Button>
      <Button size="sm" variant="primary" onclick={() => void screen.save()}>保存</Button>
    {/if}
    <IconButton label="{name} のメニュー" size="sm">⋯</IconButton>
  {/snippet}
</PageHead>
<SearchFilter {search} />
<SearchBody {search} />

<style>
  .unsaved {
    color: var(--muted);
    font-size: 12px;
  }
</style>
