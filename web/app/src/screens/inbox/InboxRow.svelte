<script lang="ts">
  import {
    type InboxRow,
    isMemberOf,
    type Member,
    pillOf,
    relativeTime,
    tokenize,
  } from "@hedgehug/core";
  import { Avatar, KindIcon, type KindName, Markdown } from "@hedgehug/ui";
  import { inbox, linkOf } from "../../inbox";
  import { links, syncNow } from "../../wire";
  import InboxAction from "./InboxAction.svelte";

  // 受信箱の 1 行(06「並びと行の表示」)。点・ピル・対象・相対時間・差出人と本文を出す。
  interface Props {
    row: InboxRow;
    // members は、行の Store のメンバー。読み終わるまでは undefined(差出人はログイン名のまま出す)。
    members: Member[] | undefined;
  }

  let { row, members }: Props = $props();

  const t = $derived(row.target);
  const link = $derived(linkOf(row));
  const member = $derived(members?.find((m) => m.login.toLowerCase() === row.sender.toLowerCase()));
  // 差出人がなければ「hedgehug」、メンバーにいなければ「不明なユーザー」と中身のないアバター(01)。
  const who = $derived(
    row.sender === "" ? "hedgehug" : (member?.name ?? (members ? "不明なユーザー" : row.sender)),
  );
  const avatar = $derived(row.sender !== "" && members && !member ? "" : who);
  const blocks = $derived(tokenize(row.text, isMemberOf(members ?? [])));
</script>

<li class="row" class:unread={!row.read}>
  <button
    type="button"
    class="dot"
    aria-label={row.read ? "未読にする" : "既読にする"}
    title={row.read ? "未読にする" : "既読にする"}
    onclick={() => void inbox.toggle(row)}
  ></button>
  <div class="main">
    <div class="line">
      <span class="pill">{pillOf(row.kind)}</span>
      {#snippet storeDot()}
        <i
          class="sdot"
          style:background="var(--t-{row.store.color}-fg)"
          role="img"
          aria-label={row.store.name}
          title={row.store.name}
        ></i>
      {/snippet}
      {#if t.kind === "deleted_item"}
        <span class="gone">削除されたアイテム</span>
      {:else if t.kind === "deleted_workspace"}
        <span class="gone">削除されたワークスペース</span>
      {:else if link}
        <a
          class="target"
          href={links.href(link.route)}
          onclick={(e) => links.click(e, link.route, link.action)}
          onauxclick={(e) => links.click(e, link.route)}
        >
          {#if t.kind === "workspace"}
            {@render storeDot()}
            <span>{row.store.name} / {t.workspaceName}</span>
          {:else}
            {#if t.kind === "item"}
              <KindIcon kind={t.itemKind as KindName} />
            {/if}
            <code class="id">{t.display}</code>
            {@render storeDot()}
            <span class="title">{t.title}</span>
          {/if}
        </a>
      {/if}
      <time class="when" datetime={row.at}>{relativeTime(new Date(row.at), $syncNow)}</time>
    </div>
    <div class="body">
      <span class="who"><Avatar name={avatar} title={who} />{who}</span>
      <Markdown {blocks} />
    </div>
    {#if row.kind === "automation_stopped"}
      <InboxAction {row} />
    {/if}
  </div>
</li>

<style>
  .row {
    display: flex;
    gap: 10px;
    padding: 10px 0;
    border-bottom: 1px solid var(--line);
  }
  .dot {
    flex: none;
    width: 18px;
    height: 18px;
    margin-top: 1px;
    border: 0;
    border-radius: 50%;
    background: none;
    cursor: pointer;
    position: relative;
  }
  .dot::after {
    content: "";
    position: absolute;
    inset: 5px;
    border-radius: 50%;
    border: 1px solid var(--line);
  }
  .unread .dot::after {
    background: var(--accent);
    border-color: var(--accent);
  }
  .dot:hover {
    background: var(--hover);
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .pill {
    font-size: 11px;
    padding: 1px 7px;
    border-radius: 999px;
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .target {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
  }
  .target:hover .title,
  .target:hover span {
    text-decoration: underline;
  }
  .unread .target {
    font-weight: 500;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .gone,
  .when {
    color: var(--muted);
  }
  .when {
    margin-left: auto;
    font-size: 12px;
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    font-size: 12.5px;
  }
</style>
