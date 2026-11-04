<script lang="ts">
  import type { Account } from "../../core/accounts";
  import { errorMessage } from "../../core/messages";
  import AccountAdd from "../accounts/AccountAdd.svelte";
  import { core } from "../context";
  import CreateStore from "../create/CreateStore.svelte";
  import StoreDiscover, {
    type Mounted,
    type MountedEvent,
    permissionLabel,
  } from "../stores/StoreDiscover.svelte";
  import { useSubscribe } from "../svelte-store";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import FormError from "../ui/FormError.svelte";

  // The first run: no Store is mounted. Three steps (the accounts, the Stores to use, the start);
  // ondone is told when the person starts, or has made a Store of their own.
  type Props = { ondone: () => void };
  let { ondone }: Props = $props();

  type Step = "accounts" | "discover" | "start";
  const STEPS: { value: Step; label: string }[] = [
    { value: "accounts", label: "アカウント" },
    { value: "discover", label: "Store を選ぶ" },
    { value: "start", label: "はじめる" },
  ];

  const c = core();
  const accounts = useSubscribe(c.accounts, () => ({
    list: c.accounts.accounts(),
    loaded: c.accounts.loaded(),
    error: c.accounts.error(),
  }));

  let step = $state<Step>("accounts");
  let creating = $state(false);
  let adding = $state(false);
  let mounted = $state<Mounted[]>([]);

  $effect(() => {
    void c.accounts.load();
  });

  // Without an account there is nothing to read or write a Store with.
  const none = $derived(accounts.current.loaded && accounts.current.list.length === 0);
  const canGo = $derived(accounts.current.loaded ? !none : accounts.current.error !== null);

  const who = (a: Account): string => {
    const slash = a.id.indexOf("/");
    return slash < 0 ? a.id : `${a.login ?? a.id.slice(slash + 1)}@${a.id.slice(0, slash)}`;
  };

  function onmounted(e: MountedEvent): void {
    mounted = [...mounted, ...e.mounted];
    // A row that failed keeps the person here with its words; the button goes on without it.
    if (e.failed === 0 && e.mounted.length > 0) step = "start";
  }
</script>

<div class="onb" data-blocking>
  <div class="brand">
    <span class="logo" aria-hidden="true">hh</span>
    <span class="welcome">Hedgehug へようこそ</span>
  </div>

  <ol class="steps" aria-label="手順">
    {#each STEPS as s, i (s.value)}
      <li aria-current={step === s.value ? "step" : undefined}>
        <span class="n">{i + 1}</span>{s.label}
      </li>
    {/each}
  </ol>

  <section class="card">
    {#if step === "accounts"}
      <h1>GitHub のアカウントを登録します</h1>
      <p>
        Hedgehug はタスクを GitHub
        のリポジトリに保存し、そのアクセス権で見える範囲を決めます。トークンはこのマシンにだけ保存します。
      </p>

      {#if accounts.current.error}
        <FormError message="アカウントを読めません · {errorMessage(accounts.current.error)}" />
        <div><Button onclick={() => c.accounts.reload()}>もう一度読む</Button></div>
      {/if}
      {#if !accounts.current.loaded}
        {#if !accounts.current.error}
          <p class="note">読み込み中…</p>
        {/if}
      {:else if accounts.current.list.length > 0}
        <ul class="accounts">
          {#each accounts.current.list as a (a.id)}
            <li class="account">
              <Avatar name={a.name} size={32} />
              <div class="who">
                <b>{a.name}</b>
                <span class="id">{who(a)}</span>
              </div>
              <span class="state" data-state={a.state}>
                {a.state === "ok" ? "有効" : a.state === "error" ? "エラー" : "トークンなし"}
              </span>
            </li>
          {/each}
        </ul>
      {/if}

      {#if none || adding}
        {#key adding}
          <AccountAdd
            onclose={none ? undefined : () => (adding = false)}
            ondone={() => (adding = false)}
          />
        {/key}
      {:else if accounts.current.loaded}
        <div><Button onclick={() => (adding = true)}>アカウントを追加</Button></div>
      {/if}

      <div class="nav">
        <span class="note">あとから 設定 › アカウントと認証 で変えられます</span>
        <span class="spacer"></span>
        {#if none}
          <span class="note">アカウントを1つ追加してください</span>
        {/if}
        <Button variant="primary" disabled={!canGo} onclick={() => (step = "discover")}>
          次へ: Store を探す
        </Button>
      </div>
    {:else if step === "discover"}
      <h1>使う Store を選んでください</h1>
      <StoreDiscover mountLabel="マウントして次へ" {onmounted} />
      <div class="nav">
        <Button onclick={() => (step = "accounts")}>戻る</Button>
        <span class="spacer"></span>
        {#if mounted.length > 0}
          <Button onclick={() => (step = "start")}>次へ</Button>
        {/if}
      </div>
    {:else}
      <h1>Store を使い始めます</h1>
      <ul class="mounted" aria-label="マウントした Store">
        {#each mounted as m (m.name)}
          <li>
            <b>{m.name}</b>
            <span class="note">{permissionLabel(m.permission)}</span>
          </li>
        {/each}
      </ul>
      <div class="nav">
        <span class="spacer"></span>
        <Button variant="primary" onclick={ondone}>始める</Button>
      </div>
    {/if}

    <div class="create">
      <Button variant="ghost" onclick={() => (creating = true)}>新しく Store を作る</Button>
    </div>
  </section>
</div>

<CreateStore open={creating} onclose={() => (creating = false)} oncreated={ondone} />

<style>
  .onb {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-20);
    padding: calc(var(--dialog-top) + var(--space-4)) var(--space-16);
    overflow: auto;
    background: var(--sidebar);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: var(--space-10);
  }
  .logo {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--logo-size);
    height: var(--logo-size);
    border-radius: var(--radius-8);
    background: var(--ink);
    color: var(--surface);
    font-family: var(--font-mono);
    font-size: var(--fs-14);
    font-weight: 500;
  }
  .welcome {
    font-size: var(--fs-17);
    font-weight: 600;
  }
  .steps {
    display: flex;
    gap: var(--space-16);
    margin: 0;
    padding: 0;
    color: var(--muted);
    font-size: var(--fs-13);
    list-style: none;
  }
  .steps li {
    display: flex;
    align-items: center;
    gap: var(--space-6);
  }
  .steps li[aria-current="step"] {
    color: var(--ink);
    font-weight: 600;
  }
  .n {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--step-size);
    height: var(--step-size);
    border-radius: var(--radius-pill);
    background: var(--sunken);
    font-size: var(--fs-12);
  }
  [aria-current="step"] .n {
    background: var(--ink);
    color: var(--surface);
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    width: 100%;
    max-width: var(--onboarding-w);
    padding: var(--space-20) var(--onboarding-pad-x);
    border-radius: var(--radius-14);
    background: var(--surface);
    box-shadow: var(--shadow-card);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  p {
    margin: 0;
    color: var(--ink-2);
    line-height: 1.7;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-13);
  }
  p.note {
    margin: 0;
  }
  .accounts,
  .mounted {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .account,
  .mounted li {
    display: flex;
    align-items: center;
    gap: var(--space-12);
    padding: var(--space-10) var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-10);
  }
  .who {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }
  .id {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .state {
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
    color: var(--muted);
  }
  .nav {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-12);
  }
  .spacer {
    flex: 1;
  }
  .create {
    display: flex;
    padding-top: var(--space-8);
    border-top: 1px solid var(--line);
  }
</style>
