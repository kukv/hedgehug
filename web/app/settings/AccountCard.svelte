<script lang="ts" module>
  export type Relogin = "device" | "token";

  const pad = (n: number) => String(n).padStart(2, "0");

  // YYYY-MM-DD HH:MM in the time zone of the browser.
  function localTime(iso: string): string {
    const d = new Date(iso);
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
</script>

<script lang="ts">
  import { type Account, envVar, source } from "../../core/accounts";
  import type { MissingApp } from "../../core/github";
  import Icon from "../icons/Icon.svelte";
  import Avatar from "../ui/Avatar.svelte";
  import Menu, { type MenuItem } from "../ui/Menu.svelte";
  import AppWarning from "./AppWarning.svelte";

  // One account: who it is, whether its token works, where the token comes from, and which Stores
  // use it. The menu has what Hedgehug can do with the token it keeps.
  type Props = {
    account: Account;
    stores: string[];
    missing?: MissingApp[];
    onrelogin: (how: Relogin) => void;
    onlogout: () => void;
  };
  let { account, stores, missing = [], onrelogin, onlogout }: Props = $props();

  // id is <host>/<login>, as config.yaml has them.
  const host = $derived(
    account.id.includes("/") ? account.id.slice(0, account.id.indexOf("/")) : "",
  );
  const who = $derived(
    host === "" ? account.id : `${account.login ?? account.id.slice(host.length + 1)}@${host}`,
  );
  const kind = $derived(source(account));
  const variable = $derived(envVar(account));

  let more: HTMLElement | null = $state(null);
  let open = $state(false);

  const items = $derived.by((): MenuItem[] => {
    switch (kind) {
      case "keychain":
        return [
          { value: "relogin-token", label: "ログインし直す" },
          { value: "logout", label: "ログアウト…" },
        ];
      case "device":
        return [
          { value: "relogin-device", label: "ログインし直す" },
          { value: "logout", label: "ログアウト…" },
        ];
      case "none":
        return [
          { value: "relogin-device", label: "GitHub でログイン" },
          { value: "relogin-token", label: "トークンを貼る" },
        ];
      default:
        return [
          {
            value: "nothing",
            label: "Hedgehug が保存しているトークンはありません",
            disabled: true,
          },
        ];
    }
  });

  function select(value: string): void {
    open = false;
    if (value === "logout") onlogout();
    else if (value === "relogin-device") onrelogin("device");
    else if (value === "relogin-token") onrelogin("token");
  }
</script>

<article class="card" aria-label={account.name}>
  <header class="top">
    <Avatar name={account.name} size={32} />
    <div class="who">
      <h3>{account.name}</h3>
      <span class="id">{who}</span>
    </div>
    <span class="state" data-state={account.state}>
      {#if account.state === "ok"}
        <Icon name="check" size={14} />有効
      {:else if account.state === "error"}
        <Icon name="warning" size={14} />エラー
      {:else}
        <Icon name="forbid" size={14} />トークンなし
      {/if}
    </span>
    <button
      type="button"
      class="more"
      bind:this={more}
      aria-label="{account.name} の操作"
      aria-haspopup="menu"
      aria-expanded={open}
      onclick={() => {
        open = !open;
      }}
    >
      <Icon name="more" size={16} />
    </button>
  </header>

  {#if account.state === "error" && account.error}
    <p class="error">{account.error}</p>
  {/if}

  <dl>
    <div class="row">
      <dt>トークンの取得元</dt>
      <dd>
        {#if kind === "gh"}
          gh
          <span class="mono"
            >gh auth token --hostname {host} --user {account.id.slice(host.length + 1)}</span
          >
        {:else if kind === "env"}
          環境変数 <span class="mono">{variable}</span>
        {:else if kind === "keychain"}
          keychain · OS の資格情報ストア
        {:else if kind === "device"}
          Device Flow · OS の資格情報ストア
          {#if account.expires}
            <span>アクセストークンの期限 {localTime(account.expires)}</span>
          {/if}
        {:else}
          なし
        {/if}
      </dd>
    </div>
    <div class="row">
      <dt>使っている Store</dt>
      <dd>{stores.length > 0 ? stores.join(" · ") : "なし"}</dd>
    </div>
  </dl>

  {#each missing as m (m.owner)}
    <AppWarning name={account.name} missing={m} canSwitch={kind !== "gh"} />
  {/each}
</article>

{#if open}
  <Menu
    anchor={more}
    {items}
    label="{account.name} の操作"
    onselect={select}
    onclose={() => {
      open = false;
    }}
  />
{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-10);
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--line);
    border-radius: var(--radius-12);
    background: var(--surface);
  }
  .top {
    display: flex;
    align-items: center;
    gap: var(--space-12);
  }
  .who {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }
  h3 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .id,
  .mono {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .state {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-4) var(--space-8);
    border-radius: var(--radius-pill);
    font-size: var(--fs-12);
  }
  .state[data-state="ok"] {
    background: var(--success-weak);
    color: var(--success-ink);
  }
  .state[data-state="error"] {
    background: var(--danger-weak);
    color: var(--danger-ink);
  }
  .state[data-state="none"] {
    background: var(--sunken);
    color: var(--ink-2);
  }
  .more {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--space-28);
    height: var(--space-28);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    cursor: pointer;
  }
  .more:hover {
    background: var(--hover);
  }
  .error {
    margin: 0;
    color: var(--danger-ink);
    font-size: var(--fs-12);
    overflow-wrap: anywhere;
  }
  dl {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    font-size: var(--fs-12);
  }
  .row {
    display: flex;
    gap: var(--space-12);
  }
  dt {
    flex: 0 0 9rem;
    color: var(--muted);
  }
  dd {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    margin: 0;
    min-width: 0;
  }
</style>
