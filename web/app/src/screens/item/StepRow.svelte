<script lang="ts">
  import {
    categoryOf,
    type DetailView,
    type Item,
    itemDisplayId,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Avatar, IconButton } from "@hedgehug/ui";
  import { detail, links } from "../../wire";
  import StatusButton from "./StatusButton.svelte";

  // Step の 1 行(02「子と Step」の「Step の行」。「×」は A4)。チェックがオンなのは分類が完了のときだけ。
  interface Props {
    v: DetailView;
    c: Item;
  }

  let { v, c }: Props = $props();
  const done = $derived(categoryOf(v.settings, c.status) === "done");
  const id = $derived(itemDisplayId(v.workspace.prefix, c.number));
  const route = $derived({ name: "item" as const, store: v.store.name, item: id });
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  const m = $derived(
    v.people.members.find((x) => x.login.toLowerCase() === c.assignee.toLowerCase()),
  );
</script>

<div class="child-row">
  <input
    type="checkbox"
    aria-label={c.title}
    checked={done}
    disabled={v.readOnly}
    title={note}
    onchange={() => void detail.checkStep(c.id, !done)}
  >
  <a
    class="row-title"
    href={links.href(route)}
    onclick={(e) => links.click(e, route, () => void detail.open(v.store.id, c.id))}
    onauxclick={(e) => links.click(e, route)}
    >{c.title}</a
  >
  <code class="id">{id}</code>
  <StatusButton
    status={c.status}
    settings={v.settings}
    disabled={v.readOnly}
    onchange={(s) => void detail.setProperty(c.id, "status", s)}
  />
  {#if c.assignee}
    <Avatar name={m?.name ?? ""} title={m?.name ?? "不明なユーザー"} />
  {/if}
  <IconButton
    label="上へ"
    size="xs"
    disabled={v.readOnly}
    title={note}
    onclick={() => void detail.moveUp(c.id)}
    >↑</IconButton
  >
</div>

<style>
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
</style>
