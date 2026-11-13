<script lang="ts">
  import { type AccountRow, askingText, methodLine } from "@hedgehug/core";
  import { Button, IconButton } from "@hedgehug/ui";

  // 一覧の 1 行(07「状態」)。表示名・@ログイン名・認証方法・状態・「編集」・削除。赤字は状態の下に出す。
  interface Props {
    row: AccountRow;
    error: string;
    asking: boolean;
    onreauth: () => void;
    onrecheck: () => void;
    onedit: () => void;
    ondelete: () => void;
  }

  let { row, error, asking, onreauth, onrecheck, onedit, ondelete }: Props = $props();
</script>

<li class="row" aria-label={row.name}>
  <strong>{row.name}</strong>
  <span>@{row.login}</span>
  <span class="muted">{methodLine(row.method)}</span>
  <span class="meta">
    {#if row.status === "auth_error"}
      <span class="pill bad">認証エラー</span>
      <Button size="xs" disabled={asking} onclick={onreauth}>再認証</Button>
    {:else if row.status === "app_missing"}
      <span class="pill warn">GitHub App 未導入</span>
      <span class="muted">{row.orgs.join("・")}</span>
      <Button size="xs" disabled={asking} onclick={onrecheck}>再確認</Button>
    {:else}
      <span class="pill ok">利用可能</span>
    {/if}
    {#if asking}
      <span class="muted">{askingText}</span>
    {/if}
    <Button size="xs" onclick={onedit}>編集</Button>
    <IconButton label="削除" size="xs" onclick={ondelete}>
      <svg width="12" height="12" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
      </svg>
    </IconButton>
  </span>
  {#if row.status === "auth_error" && row.reason}
    <p class="error">{row.reason}</p>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
</li>

<style>
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    padding: 10px 0;
    border-top: 1px solid var(--line);
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
    flex-wrap: wrap;
  }
  .pill {
    border-radius: 999px;
    padding: 1px 8px;
    font-size: 12px;
  }
  .ok {
    background: var(--c-green-bg);
    color: var(--c-green-fg);
  }
  .warn {
    background: var(--c-orange-bg);
    color: var(--c-orange-fg);
  }
  .bad {
    background: var(--c-red-bg);
    color: var(--c-red-fg);
  }
  .error {
    flex-basis: 100%;
    margin: 0;
    color: var(--danger);
    font-size: 12.5px;
  }
  svg {
    stroke: currentColor;
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
