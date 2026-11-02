<script lang="ts">
  import { cliLine } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { core } from "../context";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { openWorkspaceAdd } from "../workspace/dialogs-state.svelte";

  let { store }: { store: string } = $props();

  const { stores } = core();
  const dir = useSubscribe(stores, () => ({
    workspaces: stores.workspaces(),
    tone: stores.tone(store),
  }));
  const rows = $derived(dir.current.workspaces.filter((w) => w.store === store));
  const now = new Date();
  // The prefixes the other stores use: a prefix cannot be shared with them.
  const elsewhere = $derived(dir.current.workspaces.filter((w) => w.store !== store));
</script>

<div class="page">
  <header class="head">
    <h1>ワークスペース</h1>
    <StoreBadge {store} tone={dir.current.tone} />
    <span class="spacer"></span>
    <Button variant="primary" onclick={() => openWorkspaceAdd(store)}>ワークスペースを追加</Button>
  </header>
  <p class="lead">
    タスクを分ける単位で、それぞれが ID の接頭辞を持ちます。{store}
    を使う全員に共有されます（<span class="mono">workspaces/</span>
    への 1 コミット）。
  </p>

  <table>
    <thead>
      <tr>
        <th scope="col">名前</th>
        <th scope="col">ID の接頭辞</th>
        <th scope="col" class="num">プロジェクト</th>
        <th scope="col" class="num">タスク（削除済みを含む）</th>
        <th scope="col">接頭辞を最後に変えた日</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each rows as w (w.ulid)}
        {@const href = router.href({ name: "workspace", store, ulid: w.ulid, tab: "settings" })}
        <tr>
          <th scope="row">{w.name}</th>
          <td class="mono">{w.prefix}</td>
          <td class="num">{w.projects.length}</td>
          <td class="num">{w.tasks + w.deleted}</td>
          <td class="mono">{formatDate(w.prefixSet, now)}</td>
          <td class="end">
            <a {href} aria-label="{w.name} の設定" onclick={(e) => onLinkClick(e, href)}>設定</a>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  <p class="rule">
    接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です。同じ Store とマウント中の Store
    のワークスペースとは重ねられません。
  </p>
  {#if elsewhere.length > 0}
    <p class="rule">
      マウント中のほかの Store が使っている接頭辞:
      {#each elsewhere as w, i (`${w.store}/${w.ulid}`)}
        {i > 0 ? " · " : ""}<span class="mono">{w.prefix}</span>（{w.store}）
      {/each}
    </p>
  {/if}
  <CliLine command={cliLine(["workspace", "ls", "--store", store])} />
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    max-width: 920px;
  }
  .head {
    display: flex;
    gap: var(--space-10);
    align-items: center;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
  }
  .spacer {
    flex: 1;
  }
  .lead,
  .rule {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th,
  td {
    padding: var(--space-10) var(--space-12);
    border-bottom: 1px solid var(--line);
    text-align: left;
  }
  thead th {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  tbody th {
    font-weight: 600;
  }
  .num {
    text-align: right;
  }
  .end {
    text-align: right;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  a {
    color: var(--accent);
  }
</style>
