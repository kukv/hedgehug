<script lang="ts">
  import {
    childCount,
    type DetailView,
    type Item,
    itemDisplayId,
    type Kind,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Avatar, Button, Input, KindIcon } from "@hedgehug/ui";
  import { detail, itemDialog, links } from "../../wire";
  import StatusButton from "./StatusButton.svelte";
  import StepRow from "./StepRow.svelte";

  // 詳細パネルの「子」(02「子と Step」)。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const isEpic = $derived(v.item.kind === "Epic");
  const count = $derived(childCount(v.children, v.settings));
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  let newStep = $state("");

  const person = (it: Item) =>
    v.people.members.find((x) => x.login.toLowerCase() === it.assignee.toLowerCase());
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
  <h3>{isEpic ? "Story・Task・Bug" : "Step"} {count.done}/{count.total}</h3>
  {#each v.children as c (c.id)}
    {#if isEpic}
      {@const m = person(c)}
      <div class="child-row">
        <KindIcon kind={c.kind} />
        <code class="id">{id(c)}</code>
        <a
          class="row-title"
          href={links.href(route(c))}
          onclick={(e) => links.click(e, route(c), () => void detail.open(v.store.id, c.id))}
          onauxclick={(e) => links.click(e, route(c))}
          >{c.title}</a
        >
        <StatusButton
          status={c.status}
          settings={v.settings}
          project={c.projectId}
          disabled={v.readOnly}
          onchange={(s) => void detail.setProperty(c.id, "status", s)}
        />
        {#if c.assignee}
          <Avatar name={m?.name ?? ""} title={m?.name ?? "不明なユーザー"} />
        {/if}
      </div>
    {:else}
      <StepRow {v} {c} />
    {/if}
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
