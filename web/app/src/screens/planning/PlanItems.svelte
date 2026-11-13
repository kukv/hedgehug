<script lang="ts">
  import { deletedValue, type Member, type PlanRow, type WorkspaceSettings } from "@hedgehug/core";
  import { Avatar, KindIcon, StatusDot } from "@hedgehug/ui";
  import { detail, links } from "../../wire";

  // スプリントとリリースのカードの「アイテム {N} 件」の欄(05)。種類・ID・タイトル(押すと詳細パネル)・ポイント・
  // ステータス・担当者を出す。並べ替えの操作は付けない(並べ替えはバックログの画面で行う)。
  interface Props {
    rows: PlanRow[];
    storeId: string;
    storeName: string;
    settings: WorkspaceSettings;
    members: Member[];
    // empty は、アイテムがないときの文。
    empty?: string;
  }

  let { rows, storeId, storeName, settings, members, empty = "ありません" }: Props = $props();

  const route = (r: PlanRow) => ({ name: "item" as const, store: storeName, item: r.display });
  const statusOf = (r: PlanRow) => settings.statuses.find((s) => s.id === r.status);
</script>

{#if rows.length === 0}
  <p class="empty">{empty}</p>
{:else}
  <ul class="rows">
    {#each rows as r (r.id)}
      {@const st = statusOf(r)}
      <li class="row">
        <KindIcon kind={r.kind} />
        <code class="id">{r.display}</code>
        <a
          class="row-title"
          href={links.href(route(r))}
          onclick={(e) => links.click(e, route(r), () => void detail.open(storeId, r.id))}
          onauxclick={(e) => links.click(e, route(r))}
          >{r.title}</a
        >
        {#if r.points !== null}
          <span class="meta">{r.points} pt</span>
        {/if}
        <span class="status" class:deleted={!st}>
          {#if st}
            <StatusDot category={st.category ?? "todo"} label={st.name} />
          {/if}
          {st?.name ?? deletedValue}
        </span>
        {#if r.assignee}
          {@const m = members.find((x) => x.login.toLowerCase() === r.assignee.toLowerCase())}
          <Avatar name={m?.name ?? ""} title={m?.name ?? "不明なユーザー"} />
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .empty {
    color: var(--muted);
    font-size: 12.5px;
    margin: 4px 0;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    overflow-wrap: anywhere;
  }
  .meta,
  .status {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--ink);
  }
  .deleted {
    color: var(--muted);
  }
</style>
