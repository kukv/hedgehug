<script lang="ts">
  import {
    buildSearchTree,
    canMoveProject,
    createTreeFocus,
    get,
    type Item,
    rowKey,
    type SearchResult,
    type SearchTreeRow,
    type SearchTreeView,
    type StoreNode,
    setRowStatus,
    treeScope,
  } from "@hedgehug/core";
  import { tick } from "svelte";
  import { selection } from "../../palette";
  import { projectMover } from "../../projectRows";
  import { itemChanges } from "../../reloads";
  import { search } from "../../search";
  import {
    createAt,
    dimKeys,
    openTreeMenu,
    routeOf,
    searchTreeData,
    treeScreen,
  } from "../../searchTree";
  import { catalog, detail, itemsPort, links, navigation, toasts } from "../../wire";
  import { itemRefresh } from "../../wireActions";
  import SearchTreeBody from "./SearchTreeBody.svelte";
  import { createTreeKeyboard } from "./treeKeyboard";
  import { treeTabStops } from "./treeTabStops";

  // 検索の画面のツリー(04「ツリー」、E3a-3〜6・11・16・17・20)。結果が変わるたびに、木に入れるプロジェクトのデータを
  // 読み、読めたら木を組み直す。読んでいるあいだは前の木を出したままにする(読み直すたびに空にしない)。
  interface Props {
    // view は、描いている木と、その元の結果の書き方の誤り(件数と空の文に使う)。まだ組んでいなければ null。
    view?: (SearchTreeView & { errors: string[] }) | null;
  }

  let { view = $bindable(null) }: Props = $props();

  const searchState = search.state;
  const stores = catalog.stores;
  const result = $derived($searchState.result);
  const resultQuery = $derived($searchState.resultQuery);
  let list = $state<HTMLElement>();
  // ほかの画面にいたあいだの変更を拾うため、開いたときは取っておいたものを捨てる。
  searchTreeData.reset();
  // round は、組み直した回数。古い読み込みのあとでは組み直さない。building は、いちばん新しい組み直し。
  let round = 0;
  let building: Promise<void> = Promise.resolve();
  // rebuild は、絞り込み(応答の元。E3a-3)で木に入れるプロジェクトを読み、読めたら組み直す。最後の検索が失敗して
  // 前の結果を残しているあいだ(resultQuery が null)は、前の木のままにする。
  function rebuild(res: SearchResult | null, query: string | null, all: StoreNode[]) {
    if (!res || query === null) return building;
    const filtered = query.trim() !== "";
    const mine = ++round;
    building = searchTreeData.load(treeScope(all, res.rows, filtered)).then(() => {
      if (mine !== round) return;
      view = {
        ...buildSearchTree(all, res.rows, filtered, searchTreeData.get),
        errors: res.errors,
      };
    });
    return building;
  }
  // catalog も見る(プロジェクトの並べ替え・名前の変更は catalog だけが変わる)。
  $effect(() => void rebuild(result, resultQuery, $stores));
  // 閉じたら、読んでいる途中の組み直しを捨てる(閉じたあとで view に古い木を書き戻さない)。
  $effect(() => () => void ++round);

  // refresh は、ツリーから書いたあとの読み直し。取っておいたものを捨てて検索を読み直し、木が組み直るまで待つ
  // (順番待ちの次のキーは、組み直した木で基準を決める)。検索が失敗すると結果が変わらず上の $effect が動かない
  // (捨てたデータを読み直さない)ので、ここでも組み直す。$effect が後から組み直したら、それも待つ。
  async function refresh() {
    searchTreeData.reset();
    await search.reload();
    const s = get(search.state);
    void rebuild(s.result, s.resultQuery, get(catalog.stores));
    let last: Promise<void>;
    do {
      last = building;
      await last;
      await tick();
    } while (last !== building);
  }
  $effect(() => {
    itemChanges.add(refresh);
    return () => void itemChanges.delete(refresh);
  });

  // 開閉は画面の状態に書く(E3a-11)。開閉が変わったときだけ書く(フォーカスの移動では書かない)。
  const treeFocus = createTreeFocus<SearchTreeRow>(get(search.state).tree.collapsed);
  let written = get(treeFocus.state).collapsed;
  $effect(() =>
    treeFocus.state.subscribe((f) => {
      if (f.collapsed === written) return;
      written = f.collapsed;
      search.setCollapsed([...f.collapsed]);
    }),
  );
  const rowAt = (key: string | null) => get(treeFocus.state).rows.find((r) => r.id === key) ?? null;
  const setStatus = (it: Item, statusId: string) =>
    setRowStatus({ items: itemsPort, toasts, reload: refresh }, it.storeId, it.id, statusId);
  const openMenu = (row: SearchTreeRow, rect: { left: number; bottom: number }) =>
    openTreeMenu(row, rect, get(treeFocus.state).rows, view);

  const keyboard = createTreeKeyboard<SearchTreeRow>({
    focus: treeFocus,
    list: () => list,
    itemOf: (row) => (row.kind === "item" ? row.item : null),
    screen: () => treeScreen(get(treeFocus.state), view),
    readOnly: (row) => row.store.readOnly,
    open: (row) => {
      if (row.kind === "item") void detail.open(row.store.id, row.item.id);
      else navigation.go(routeOf(row));
    },
    openTab: (row) => links.openTab(routeOf(row)),
    openMenu,
    create: createAt,
    setStatus,
    // サイドバーの並びの先頭・末尾では何もしない(E3a-13・16)。
    moveProject: (row, dir) => {
      if (row.kind === "project" && canMoveProject(row.workspace, row.project.id, dir)) {
        void projectMover.move(row.store.id, row.project.id, dir);
      }
    },
    // 並べ替え・移動のあとの読み直し。検索のツリーは itemChanges(refresh)で読み直す。
    afterChange: () => itemRefresh.afterChange(),
    // 新しい親は内部 ID で来る。フォーカスのある行と同じ Store の行の鍵にする。
    keyOf: (id) => rowKey(rowAt(get(treeFocus.state).currentId)?.store.id ?? "", id),
  });
  const focus = keyboard.focus.state;
  // 木が変わったときは、見えている行とフォーカスのある行を直す。描く前に済ませる。
  $effect.pre(() => {
    if (view) keyboard.focus.setTree(view.nodes);
  });
  $effect(() => () => keyboard.destroy());
  const dim = $derived(view ? dimKeys(view.nodes) : new Set<string>());

  // パレットの「選んでいるアイテム」は、ツリーにフォーカスがあり、フォーカスのある行がアイテムの行なら、そのアイテム(E3a-20)。
  $effect(() =>
    selection.provide(1, () => {
      const row = rowAt(get(keyboard.focus.state).currentId);
      if (row?.kind !== "item" || !list?.contains(document.activeElement)) return null;
      const it = row.item;
      return {
        storeId: it.storeId,
        itemId: it.id,
        source: "tree",
        setStatus: (s) => setStatus(it, s),
      };
    }),
  );
</script>

{#if view && view.nodes.length > 0}
  <!-- ツリーは Tab で入る 1 つのフォーカス先で、中の行は矢印で動かす(04「ツリーのキー操作」) -->
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <ul
    class="tree"
    aria-label="ツリー"
    bind:this={list}
    use:treeTabStops
    onkeydown={keyboard.onkeydown}
    onfocus={keyboard.onfocus}
    onfocusout={keyboard.onfocusout}
    onpointerdown={keyboard.onpointerdown}
  >
    <SearchTreeBody
      {view}
      collapsed={$focus.collapsed}
      focusedId={$focus.currentId}
      {dim}
      ontoggle={keyboard.focus.toggle}
      onstatus={(it, id) => void setStatus(it, id)}
      onmenu={(key, rect) => {
        const row = rowAt(key);
        if (row) openMenu(row, rect);
      }}
    />
  </ul>
{/if}

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  /* フォーカスのある行は、行全体を囲む枠で示す(行の選択ではない。ツリーにフォーカスがあるあいだだけ) */
  .tree:focus-within :global(.tree-row.focused) {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
</style>
