<script lang="ts" module>
  export type Mounted = { name: string; permission: string };
  export type MountedEvent = { mounted: Mounted[]; failed: number };

  // The owner and the repo of a clone URL (https://host/owner/repo.git, git@host:owner/repo.git);
  // null for a path or anything else.
  export function parseRepo(url: string): { owner: string; repo: string } | null {
    const m = /^(?:[a-z+]+:\/\/)?(?:[^@/]+@)?[^/:]+[/:]([^/]+)\/([^/]+?)(?:\.git)?\/?$/i.exec(
      url.trim(),
    );
    return m?.[1] && m[2] ? { owner: m[1], repo: m[2] } : null;
  }

  const OTHER = "その他";

  export function permissionLabel(permission: string): string {
    if (permission === "write") return "書き込み";
    if (permission === "read") return "閲覧のみ";
    return permission;
  }
</script>

<script lang="ts">
  import { source } from "../../core/accounts";
  import { ApiError, type Schemas, unwrap } from "../../core/api/client";
  import { cliStoreDiscover } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";

  // Finds the Stores an account can read (or the ones in a folder) and mounts the ones chosen.
  // The Onboarding and the Settings both use it; onmounted is told what one mount did.
  type Props = { mountLabel: string; onmounted?: (e: MountedEvent) => void };
  let { mountLabel, onmounted }: Props = $props();

  type Row = {
    c: Schemas["Candidate"];
    selected: boolean;
    nick: string;
    mounted: boolean;
    nameError: boolean;
    error: string | null;
  };

  const c = core();
  const uid = $props.id();
  const accounts = useSubscribe(c.accounts, () => ({
    list: c.accounts.accounts(),
    loaded: c.accounts.loaded(),
    error: c.accounts.error(),
  }));

  let rows = $state<Row[]>([]);
  // idle: nothing looked yet; choose: the server wants to know which account
  let phase = $state<"idle" | "loading" | "done" | "error" | "choose">("idle");
  let failure = $state<string | null>(null);
  let folder = $state("");
  let account = $state("");
  // The folder the rows came from ("" for GitHub).
  let looked = $state("");
  let choices = $state<string[] | null>(null);
  let busy = $state(false);
  let mountFailure = $state<string | null>(null);
  let seq = 0;
  let auto = false;

  const withToken = $derived(
    accounts.current.list.filter((a) => source(a) !== "none" && a.state === "ok"),
  );
  const cli = $derived(
    cliStoreDiscover({ account: account || undefined, from: folder.trim() || undefined }),
  );
  const chosen = $derived(rows.filter((r) => r.selected && !r.mounted));
  const groups = $derived.by(() => {
    const by = new Map<string, Row[]>();
    for (const r of rows) {
      const owner = parseRepo(r.c.url)?.owner ?? OTHER;
      by.set(owner, [...(by.get(owner) ?? []), r]);
    }
    return [...by].map(([owner, list]) => ({
      owner,
      accounts: [...new Set(list.map((r) => r.c.account).filter((a): a is string => !!a))],
      list,
    }));
  });

  const repoName = (url: string): string => {
    const p = parseRepo(url);
    return p ? `${p.owner}/${p.repo}` : url;
  };

  // The accounts are read when this opens: the server asks GitHub for them, so nothing else does.
  $effect(() => {
    void c.accounts.load();
  });

  // GitHub is searched on its own once there is an account to search with.
  $effect(() => {
    if (auto || !accounts.current.loaded || withToken.length === 0) return;
    auto = true;
    void search("");
  });

  async function search(from: string): Promise<void> {
    const mine = ++seq;
    phase = "loading";
    // Rows of an older search are not on the screen, so they cannot be chosen or mounted.
    rows = [];
    failure = null;
    mountFailure = null;
    const query: { account?: string; from?: string } = {};
    if (account) query.account = account;
    if (from) query.from = from;
    try {
      const found = await unwrap(c.api.GET("/api/stores/discover", { params: { query } }));
      if (mine !== seq) return;
      rows = (found ?? []).map((x) => ({
        c: x,
        selected: false,
        nick: x.name,
        mounted: false,
        nameError: false,
        error: null,
      }));
      looked = from;
      choices = null;
      phase = "done";
    } catch (e) {
      if (mine !== seq) return;
      if (e instanceof ApiError && e.type === "choice.required" && e.args.what === "--account") {
        const names = e.args.candidates;
        choices = Array.isArray(names) ? names.map(String) : [];
        phase = "choose";
        return;
      }
      failure = errorMessage(e);
      phase = "error";
    }
  }

  function searchFolder(): void {
    void search(folder.trim());
  }

  async function mount(): Promise<void> {
    if (busy || phase !== "done") return;
    const sent: Row[] = [];
    let unnamed = 0;
    for (const r of chosen) {
      r.nameError = r.nick.trim() === "";
      if (r.nameError) unnamed++;
      else sent.push(r);
    }
    if (sent.length === 0) return;
    busy = true;
    mountFailure = null;
    try {
      const body = sent.map((r) => ({
        name: r.nick.trim(),
        url: r.c.url,
        account: r.c.account || undefined,
      }));
      const res = await unwrap(c.api.POST("/api/stores/mount", { body }));
      const mounted = [...(res.mounted ?? [])];
      const failed = [...(res.failed ?? [])];
      const done: Mounted[] = [];
      for (const [i, r] of sent.entries()) {
        const name = body[i]?.name ?? "";
        const k = mounted.indexOf(name);
        if (k >= 0) {
          mounted.splice(k, 1);
          r.mounted = true;
          r.selected = false;
          r.error = null;
          done.push({ name, permission: r.c.permission });
          continue;
        }
        const f = failed.findIndex((x) => x.name === name);
        if (f >= 0) {
          const [x] = failed.splice(f, 1);
          r.error = errorMessage(
            new ApiError({ status: 400, type: x?.code ?? "", detail: x?.message, args: { name } }),
          );
        }
      }
      if (done.length > 0) await c.stores.reload().catch(() => undefined);
      onmounted?.({ mounted: done, failed: (res.failed?.length ?? 0) + unnamed });
    } catch (e) {
      mountFailure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<div class="discover">
  {#if !accounts.current.loaded && !accounts.current.error}
    <p class="note">読み込み中…</p>
  {:else if accounts.current.loaded && withToken.length === 0 && phase === "idle"}
    <p class="note">GitHub で探すにはトークンのあるアカウントが要ります</p>
  {/if}

  {#if phase === "loading"}
    <p class="note">探しています…</p>
  {:else if phase === "error"}
    <FormError message={failure ?? ""} />
    <div><Button disabled={busy} onclick={() => search(looked)}>もう一度探す</Button></div>
  {:else if phase === "done"}
    <p class="note">
      {#if looked}
        フォルダ {looked} の下から、Store を持つ bare リポジトリを探しました。
      {:else}
        あなたのアカウントと所属する Org から、topic hedgehug-store
        が付いたリポジトリを探しました。読めないリポジトリはここに出てきません。
      {/if}
    </p>
    {#if rows.length === 0}
      <div class="none">
        <b>見つかりませんでした</b>
        <span class="note">
          探しているチームの Store
          が見つからないときは、そのリポジトリの権限をチームの管理者に依頼してください。
        </span>
      </div>
    {:else}
      {#each groups as g (g.owner)}
        <section class="group" aria-label={g.owner}>
          <h3>
            <span>{g.owner}</span>
            {#if g.accounts.length > 0}
              <small>{g.accounts.join("・")}</small>
            {/if}
          </h3>
          <ul>
            {#each g.list as r (r.c.url)}
              <li class="row" class:done={r.mounted}>
                <input
                  type="checkbox"
                  bind:checked={r.selected}
                  disabled={r.mounted}
                  aria-label="{repoName(r.c.url)} を選ぶ"
                >
                <span class="repo">{repoName(r.c.url)}</span>
                <span class="perm">{permissionLabel(r.c.permission)}</span>
                <input
                  class="nick"
                  type="text"
                  bind:value={r.nick}
                  disabled={r.mounted}
                  aria-label="{repoName(r.c.url)} の呼び名"
                  aria-invalid={r.nameError ? "true" : undefined}
                  oninput={() => (r.nameError = false)}
                >
                {#if r.mounted}
                  <span class="msg ok">マウントしました</span>
                {:else if r.nameError}
                  <span class="msg bad">名前を入れてください</span>
                {:else if r.error}
                  <span class="msg bad">{r.error}</span>
                {/if}
              </li>
            {/each}
          </ul>
        </section>
      {/each}
    {/if}
  {/if}

  <section class="folder" aria-label="フォルダから探す">
    <h3>フォルダから探す</h3>
    <div class="line">
      <div class="field">
        <label for="{uid}-folder">フォルダ</label>
        <input
          id="{uid}-folder"
          class="mono"
          type="text"
          bind:value={folder}
          placeholder="/srv/git"
          autocomplete="off"
        >
      </div>
      {#if choices}
        <div class="field">
          <label for="{uid}-account">アカウント</label>
          <select id="{uid}-account" bind:value={account}>
            <option value="">選ぶ</option>
            {#each choices as a (a)}
              <option value={a}>{a}</option>
            {/each}
          </select>
        </div>
      {/if}
      <Button onclick={searchFolder} disabled={busy || folder.trim() === ""}> 探す </Button>
    </div>
    {#if phase === "choose"}
      <p class="note">どのアカウントで探すかを選んでください。</p>
    {/if}
  </section>

  {#if mountFailure}
    <FormError message={mountFailure} />
  {/if}
  <div class="foot">
    <CliLine command={cli} />
    <Button
      variant="primary"
      disabled={busy || phase !== "done" || chosen.length === 0}
      onclick={mount}
    >
      {mountLabel}
    </Button>
  </div>
</div>

<style>
  .discover {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
    line-height: 1.6;
  }
  .none {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-16);
    border: 1px dashed var(--line);
    border-radius: var(--radius-10);
  }
  h3 {
    display: flex;
    align-items: baseline;
    gap: var(--space-8);
    margin: 0 0 var(--space-6);
    font-size: var(--fs-13);
    font-weight: 600;
  }
  h3 small {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 400;
  }
  ul {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto minmax(0, 12rem);
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-6) var(--space-8);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
  }
  .row.done {
    background: var(--accent-weak);
  }
  .repo {
    overflow: hidden;
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .perm {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .msg {
    grid-column: 2 / -1;
    font-size: var(--fs-12);
  }
  .msg.bad {
    color: var(--danger-ink);
  }
  .msg.ok {
    color: var(--ink-2);
  }
  input[type="text"],
  select {
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    color: var(--ink);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .folder {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding-top: var(--space-8);
    border-top: 1px solid var(--line);
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-8);
  }
  .field {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 10rem;
  }
  label {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-8);
  }
</style>
