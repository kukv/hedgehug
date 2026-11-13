<script lang="ts">
  import {
    type DetailView,
    type RelationLabel,
    type RelationRow,
    readOnlyNote,
    relationLabels,
  } from "@hedgehug/core";
  import { Button, IconButton, Input, Select } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { detail, links, relations } from "../../wire";
  import RelationCandidates from "./RelationCandidates.svelte";

  // 詳細パネルの「関連」(02「関連」)。一覧は表記ごとに見出しを付け、相手を押すとその詳細パネルを開く。
  // 追加は、表記のセレクト・相手の検索・「追加」。閲覧のみの Store では、足す・外すを無効にする。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();
  const model = relations.state;
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  const options = relationLabels.map((l) => ({ value: l, label: l }));
  const route = (r: RelationRow) => ({
    name: "item" as const,
    store: v.store.name,
    item: r.display,
  });
  // 開いているアイテムが変わったときだけ読み直す(詳細パネルの読み直しでは、wire.ts の reflect と
  // 関連の追加・解除が、一覧だけを読み直す)。
  const storeId = $derived(v.store.id);
  const itemId = $derived(v.item.id);

  $effect(() => {
    void relations.load(storeId, itemId);
  });
  onDestroy(() => relations.reset());

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing && $model.canAdd) void relations.add();
  }
</script>

<section class="peek-sec" aria-label="関連">
  <h3>関連</h3>
  {#if $model.loaded && $model.groups.length === 0}
    <p class="muted">関連はありません</p>
  {/if}
  {#each $model.groups as g (g.label)}
    <h4>{g.label}</h4>
    {#each g.rows as r (r.id)}
      <div class="rel-row">
        <code class="id">{r.display}</code>
        <a
          class="row-title"
          href={links.href(route(r))}
          onclick={(e) => links.click(e, route(r), () => void detail.open(storeId, r.id))}
          onauxclick={(e) => links.click(e, route(r))}
          >{r.title}</a
        >
        {#if r.workspaceName}
          <span class="muted">{r.workspaceName}</span>
        {/if}
        {#if r.statusName}
          <span
            class="status"
            style="--bg: var(--c-{r.statusColor}-bg); --fg: var(--c-{r.statusColor}-fg)"
            >{r.statusName}</span
          >
        {/if}
        <IconButton
          label="{g.label} {r.display} を解除"
          size="xs"
          disabled={v.readOnly}
          title={note}
          onclick={() => void relations.remove(r)}
          >×</IconButton
        >
      </div>
    {/each}
  {/each}
  <div class="add-row">
    <Select
      label="関連の表記"
      size="sm"
      {options}
      value={$model.label}
      disabled={v.readOnly}
      title={note}
      onchange={(l) => void relations.setLabel(l as RelationLabel)}
    />
    <Input
      label="相手"
      size="sm"
      placeholder="ID またはタイトルで検索"
      value={$model.query}
      disabled={v.readOnly}
      title={note}
      oninput={(q) => void relations.setQuery(q)}
      {onkeydown}
    />
    <Button
      size="sm"
      disabled={v.readOnly || !$model.canAdd || $model.busy}
      title={note}
      onclick={() => void relations.add()}
      >追加</Button
    >
  </div>
  <RelationCandidates candidates={$model.candidates} onchoose={(c) => relations.choose(c)} />
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
  h4 {
    font-size: 11.5px;
    font-weight: 600;
    color: var(--muted);
    margin: 4px 0 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .rel-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
  }
  .row-title:hover {
    text-decoration: underline;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .status {
    padding: 0 6px;
    border-radius: 9px;
    font-size: 11.5px;
    background: var(--bg);
    color: var(--fg);
    white-space: nowrap;
  }
  .add-row {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .add-row :global(input) {
    flex: 1;
  }
</style>
