<script lang="ts">
  import {
    type DetailView,
    type Item,
    itemDisplayId,
    type Kind,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, IconButton, Input, KindIcon } from "@hedgehug/ui";
  import { detail, itemDialog, links } from "../../wire";

  // 詳細パネルの「子」(02「子と Step」のうち、一覧・Step の追加・「上へ」)。完了の数・ステータス・担当者・
  // Step のチェックと「×」は A3・A4(A2 の計画の決定 18)。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const isEpic = $derived(v.item.kind === "Epic");
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  let newStep = $state("");

  const id = (it: Item) => itemDisplayId(v.workspace.prefix, it.number);
  const route = (it: Item) => ({ name: "item" as const, store: v.store.name, item: id(it) });

  function add(kind: Kind) {
    void itemDialog.open({
      workspaceId: v.workspace.id,
      projectId: v.project.id,
      kind,
      parentId: v.item.id,
    });
  }

  async function onkeydown(event: KeyboardEvent) {
    if (event.key !== "Enter" || event.isComposing) return;
    // 追加したら入力欄を空にし、フォーカスはそのまま残す(02「子と Step」)。
    if (await detail.addStep(newStep)) newStep = "";
  }
</script>

<section class="peek-sec" aria-label={isEpic ? "Story・Task・Bug" : "Step"}>
  <h3>{isEpic ? "Story・Task・Bug" : "Step"}</h3>
  {#each v.children as c (c.id)}
    <div class="child-row">
      {#if isEpic}
        <KindIcon kind={c.kind} />
        <code class="id">{id(c)}</code>
      {/if}
      <a
        class="row-title"
        href={links.href(route(c))}
        onclick={(e) => links.click(e, route(c), () => void detail.open(v.store.id, c.id))}
        onauxclick={(e) => links.click(e, route(c))}
        >{c.title}</a
      >
      {#if !isEpic}
        <code class="id">{id(c)}</code>
        <IconButton
          label="上へ"
          size="xs"
          disabled={v.readOnly}
          title={note}
          onclick={() => void detail.moveUp(c.id)}
          >↑</IconButton
        >
      {/if}
    </div>
  {/each}
  {#if isEpic}
    <div class="row-gap">
      {#each ["Story", "Task", "Bug"] as const as k (k)}
        <Button size="sm" disabled={v.readOnly} title={note} onclick={() => add(k)}>
          <span aria-hidden="true">+</span> {k}
        </Button>
      {/each}
    </div>
  {:else}
    <Input
      label="Step を追加"
      placeholder="Step を追加して Enter"
      disabled={v.readOnly}
      title={note}
      {onkeydown}
      bind:value={newStep}
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
  .child-row {
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
  .row-gap {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
</style>
