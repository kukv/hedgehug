<script lang="ts">
  import { onMount } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliAuthLogout } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import AccountAdd, { type AddSource } from "../accounts/AccountAdd.svelte";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import AccountCard, { type Relogin } from "./AccountCard.svelte";

  const c = core();
  const accounts = useSubscribe(c.accounts, () => ({
    list: c.accounts.accounts(),
    loaded: c.accounts.loaded(),
    error: c.accounts.error(),
  }));
  const stores = useSubscribe(c.stores, () => c.stores.stores());

  // The form that adds an account, or logs one in again with its name and host filled in. A new
  // key makes a form of its own for each opening.
  let adding = $state.raw<{
    key: number;
    initial: { source?: AddSource; name?: string; host?: string };
  } | null>(null);
  let opened = 0;

  let leaving = $state.raw<{ name: string } | null>(null);
  let leaveFailure = $state<string | null>(null);
  let leaveBusy = $state(false);

  onMount(() => {
    void c.accounts.load();
  });

  function openAdd(initial: { source?: AddSource; name?: string; host?: string } = {}): void {
    adding = { key: ++opened, initial };
  }

  function relogin(name: string, id: string, how: Relogin): void {
    const slash = id.indexOf("/");
    openAdd({
      source: how === "device" ? "device" : "token",
      name,
      host: slash > 0 ? id.slice(0, slash) : undefined,
    });
  }

  function askLogout(name: string): void {
    leaveFailure = null;
    leaving = { name };
  }

  async function logout(): Promise<void> {
    const target = leaving;
    if (!target || leaveBusy) return;
    leaveBusy = true;
    leaveFailure = null;
    try {
      await unwrap(
        c.api.DELETE("/api/accounts/{name}/token", { params: { path: { name: target.name } } }),
      );
      leaving = null;
      toast({ message: `${target.name} からログアウトしました` });
      await Promise.all([c.accounts.reload(), c.stores.reload().catch(() => undefined)]);
    } catch (e) {
      leaveFailure = errorMessage(e);
    } finally {
      leaveBusy = false;
    }
  }

  const usedBy = (name: string): string[] =>
    stores.current.filter((s) => s.account === name).map((s) => s.name);
</script>

<div class="page">
  <header class="head">
    <h1>アカウントと認証</h1>
    <span class="spacer"></span>
    {#if !(accounts.current.loaded && accounts.current.list.length === 0 && !adding)}
      <Button variant="primary" onclick={() => openAdd()}>アカウントを追加</Button>
    {/if}
  </header>
  <p class="lead">
    ここに登録した全アカウントを「自分」として扱います（<code>assignee:@me</code
    >）。トークンはこのマシンにだけ保存し、Store には書きません。
  </p>

  {#if accounts.current.error}
    <FormError message="アカウントを読めません · {errorMessage(accounts.current.error)}" />
    <div><Button onclick={() => c.accounts.reload()}>もう一度読む</Button></div>
  {/if}

  {#if !accounts.current.loaded}
    {#if !accounts.current.error}
      <p class="note">読み込み中…</p>
    {/if}
  {:else if accounts.current.list.length === 0}
    <div class="none">
      <b>アカウントがありません</b>
      {#if !adding}
        <Button variant="primary" onclick={() => openAdd()}>アカウントを追加</Button>
      {/if}
    </div>
  {:else}
    <ul class="cards">
      {#each accounts.current.list as a (a.id)}
        <li>
          <AccountCard
            account={a}
            stores={usedBy(a.name)}
            onrelogin={(how) => relogin(a.name, a.id, how)}
            onlogout={() => askLogout(a.name)}
          />
        </li>
      {/each}
    </ul>
  {/if}

  {#if adding}
    {#key adding.key}
      <AccountAdd
        initial={adding.initial}
        onclose={() => {
          adding = null;
        }}
        ondone={() => {
          adding = null;
        }}
      />
    {/key}
  {/if}
</div>

<Dialog
  open={leaving !== null}
  title="{leaving?.name ?? ""} からログアウト"
  width={560}
  cli={leaving ? cliAuthLogout(leaving.name) : undefined}
  onclose={() => {
    leaving = null;
  }}
>
  <p class="confirm">
    {leaving?.name}
    からログアウトします。keychain のトークンを消します。アカウントは config.yaml に残ります。
  </p>
  {#if leaveFailure}
    <FormError message={leaveFailure} />
  {/if}
  {#snippet footer()}
    <Button
      onclick={() => {
        leaving = null;
      }}
      >キャンセル</Button
    >
    <Button variant="danger" disabled={leaveBusy} onclick={logout}>ログアウト</Button>
  {/snippet}
</Dialog>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    max-width: 48rem;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-12);
  }
  .spacer {
    flex: 1;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  code {
    font-family: var(--font-mono);
  }
  .lead,
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .cards {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .none {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-10);
    padding: 32px var(--space-20);
    color: var(--ink-2);
  }
  .confirm {
    margin: 0 0 var(--space-12);
    line-height: 1.6;
  }
</style>
