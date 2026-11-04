<script lang="ts" module>
  export type AddSource = "device" | "token" | "gh" | "env" | "none";

  // The name of an environment variable the token is read from.
  const ENV_NAME = /^[A-Z0-9_]+$/;

  const SOURCES: { value: AddSource; label: string; note: string }[] = [
    {
      value: "device",
      label: "Device Flow",
      note: "Hedgehug 専用の GitHub App でログイン。gh がなくても使える",
    },
    {
      value: "token",
      label: "トークンを貼る",
      note: "自分で発行したトークンを OS の資格情報ストアに保存",
    },
    {
      value: "gh",
      label: "gh",
      note: "gh の複数アカウントをそのまま使う。gh があるときの第一候補",
    },
    { value: "env", label: "環境変数", note: "指定した変数から読む。CI 向け" },
    {
      value: "none",
      label: "トークンなし",
      note: "トークンを使わないアカウント。認証なしで届く remote の Store だけを使う",
    },
  ];
</script>

<script lang="ts">
  import { ApiError, type Schemas, unwrap } from "../../core/api/client";
  import { cliAuthAdd, cliAuthLogin, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";
  import DeviceFlow from "./DeviceFlow.svelte";

  // Adds an account, or logs one in again: the five ways to get its token and the form of each.
  // The Settings screen and the Onboarding both use it. ondone is told the name of the account.
  type Props = {
    initial?: { source?: AddSource; name?: string; host?: string };
    onclose?: () => void;
    ondone?: (account: string) => void;
  };
  let { initial = {}, onclose, ondone }: Props = $props();

  const c = core();
  const uid = $props.id();

  // svelte-ignore state_referenced_locally
  let source = $state<AddSource>(initial.source ?? "device");
  // svelte-ignore state_referenced_locally
  let name = $state(initial.name ?? "");
  // svelte-ignore state_referenced_locally
  let host = $state(initial.host ?? "github.com");
  let login = $state("");
  let envName = $state("");
  let token = $state("");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  // The Device Flow that is running, and one that the server has but this screen cannot take over.
  let flow = $state.raw<Schemas["DeviceStartBody"] | null>(null);
  let resumed = $state(false);
  let stale = $state<string | null>(null);
  // The accounts there were when the flow began: a login of one of them is a new login, not an addition.
  let known = new Set<string>();
  // A newer start, or a change of the source, makes the answer of an older start be dropped.
  let seq = 0;

  const trimmed = $derived({ name: name.trim(), host: host.trim(), login: login.trim() });
  const envOk = $derived(ENV_NAME.test(envName));
  const complete = $derived.by(() => {
    switch (source) {
      case "device":
        return true;
      case "token":
        return trimmed.name !== "" && token.trim() !== "";
      case "env":
        return trimmed.name !== "" && trimmed.login !== "" && envOk;
      default:
        return trimmed.name !== "" && trimmed.login !== "";
    }
  });

  // The hh line of the chosen way. The token is never part of it.
  const cli = $derived.by(() => {
    const n = trimmed.name || new Placeholder("名前");
    if (source === "device")
      return cliAuthLogin({ name: trimmed.name || undefined, host: trimmed.host });
    if (source === "token") return cliAuthLogin({ name: n, host: trimmed.host, withToken: true });
    return cliAuthAdd({
      name: n,
      login: trimmed.login || new Placeholder("login"),
      host: trimmed.host,
      token: source === "gh" ? "gh" : source === "env" ? `env:${envName || "<変数名>"}` : undefined,
    });
  });

  async function remove(id: string): Promise<void> {
    await unwrap(c.api.DELETE("/api/auth/device/{id}", { params: { path: { id } } }));
  }

  // Stops the flow that is running. A failure does not matter: the flow is over or ends by itself.
  function cancel(): void {
    seq++;
    const f = flow;
    flow = null;
    stale = null;
    if (f) remove(f.id).catch(() => undefined);
  }

  $effect(() => cancel);

  function choose(next: AddSource): void {
    if (next === source) return;
    cancel();
    failure = null;
    token = "";
    source = next;
  }

  async function refresh(): Promise<void> {
    await Promise.all([c.accounts.reload(), c.stores.reload().catch(() => undefined)]);
  }

  function added(account: string): void {
    void refresh();
    ondone?.(account);
  }

  async function start(): Promise<void> {
    const mine = ++seq;
    failure = null;
    stale = null;
    busy = true;
    known = new Set(c.accounts.accounts().map((a) => a.name));
    try {
      const started = await unwrap(
        c.api.POST("/api/auth/device", {
          body: { name: trimmed.name || undefined, host: trimmed.host || undefined },
        }),
      );
      if (mine !== seq) {
        remove(started.id).catch(() => undefined);
        return;
      }
      flow = started;
      resumed = false;
    } catch (e) {
      if (mine !== seq) return;
      if (e instanceof ApiError && e.type === "auth.device_running") {
        const a = e.args;
        const id = typeof a.id === "string" ? a.id : "";
        if (typeof a.user_code === "string" && typeof a.expires_at === "string") {
          flow = {
            id,
            user_code: a.user_code,
            verification_uri: typeof a.verification_uri === "string" ? a.verification_uri : "",
            expires_at: a.expires_at,
          };
          resumed = true;
        } else {
          stale = id;
        }
      } else {
        failure = errorMessage(e);
      }
    } finally {
      if (mine === seq) busy = false;
    }
  }

  // The flow that is running elsewhere is stopped, and ours begins.
  async function restart(): Promise<void> {
    const mine = ++seq;
    const old = flow?.id ?? stale;
    flow = null;
    stale = null;
    failure = null;
    if (old) {
      try {
        await remove(old);
      } catch (e) {
        if (mine !== seq) return;
        if (!(e instanceof ApiError && e.type === "auth.device_not_found")) {
          failure = errorMessage(e);
          return;
        }
      }
      // the source was changed or the form went while the old flow was being stopped
      if (mine !== seq) return;
    }
    await start();
  }

  function deviceDone(account: string): void {
    flow = null;
    toast({
      message: known.has(account)
        ? `${account} にログインしました`
        : `アカウント ${account} を追加しました`,
    });
    added(account);
  }

  async function paste(): Promise<void> {
    const body = { token: token.trim(), host: trimmed.host || undefined };
    // the token is not kept: a failure does not keep it either
    token = "";
    busy = true;
    failure = null;
    try {
      const r = await unwrap(
        c.api.PUT("/api/accounts/{name}/token", {
          params: { path: { name: trimmed.name } },
          body,
        }),
      );
      toast({
        message: r.uses
          ? `${r.account} は ${r.uses} からトークンを得るため、保存したものはありません`
          : r.added
            ? `アカウント ${r.account} を追加しました`
            : `${r.account} のトークンを保存しました`,
      });
      added(r.account);
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function add(): Promise<void> {
    busy = true;
    failure = null;
    try {
      await unwrap(
        c.api.POST("/api/accounts", {
          body: {
            name: trimmed.name,
            login: trimmed.login,
            host: trimmed.host || undefined,
            token: source === "gh" ? "gh" : source === "env" ? `env:${envName}` : undefined,
          },
        }),
      );
      toast({ message: `アカウント ${trimmed.name} を追加しました` });
      const account = trimmed.name;
      name = "";
      login = "";
      envName = "";
      added(account);
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function submit(e: SubmitEvent): void {
    e.preventDefault();
    if (!complete || busy || flow || stale !== null) return;
    if (source === "device") void start();
    else if (source === "token") void paste();
    else void add();
  }

  const submitLabel = $derived(
    source === "device" ? "GitHub でログイン" : source === "token" ? "トークンを保存" : "追加する",
  );
</script>

<section class="add" aria-label="アカウントを追加">
  <header class="head">
    <h2>アカウントを追加</h2>
    {#if onclose}
      <Button variant="ghost" size="sm" onclick={onclose}>閉じる</Button>
    {/if}
  </header>

  <div class="sources" role="radiogroup" aria-label="トークンの取得元">
    {#each SOURCES as s (s.value)}
      <label class="source" class:on={source === s.value}>
        <input
          type="radio"
          name="{uid}-source"
          value={s.value}
          checked={source === s.value}
          aria-labelledby="{uid}-{s.value}-label"
          aria-describedby="{uid}-{s.value}-note"
          onchange={() => choose(s.value)}
        >
        <span class="label" id="{uid}-{s.value}-label">{s.label}</span>
        <span class="note" id="{uid}-{s.value}-note">{s.note}</span>
      </label>
    {/each}
  </div>

  <form onsubmit={submit}>
    <fieldset disabled={busy || flow !== null || stale !== null}>
      <label class="field">
        <span>呼び名{source === "device" ? "（任意。空なら login）" : ""}</span>
        <input type="text" bind:value={name} autocomplete="off" required={source !== "device"}>
      </label>
      {#if source === "gh" || source === "env" || source === "none"}
        <label class="field">
          <span>login</span>
          <input type="text" bind:value={login} autocomplete="off" required>
        </label>
      {/if}
      <label class="field">
        <span>ホスト</span>
        <input type="text" bind:value={host} autocomplete="off">
      </label>
      {#if source === "env"}
        <div class="field">
          <label for="{uid}-env">変数名</label>
          <input
            id="{uid}-env"
            type="text"
            bind:value={envName}
            autocomplete="off"
            required
            aria-invalid={envName !== "" && !envOk ? "true" : undefined}
            aria-describedby="{uid}-env-hint"
          >
          <small id="{uid}-env-hint" class:bad={envName !== "" && !envOk}>
            英大文字・数字・_ だけ使えます
          </small>
        </div>
      {/if}
      {#if source === "token"}
        <label class="field">
          <span>トークン</span>
          <input type="password" bind:value={token} autocomplete="off" required>
        </label>
      {/if}
    </fieldset>

    {#if failure}
      <FormError message={failure} />
    {/if}

    {#if stale !== null}
      <div class="stale">
        <p>ログインが別の画面で始まっています。</p>
        <Button onclick={restart} disabled={busy}>やめてやり直す</Button>
      </div>
    {:else if flow}
      <DeviceFlow {flow} {resumed} ondone={deviceDone} onretry={restart} />
    {:else}
      <div>
        <Button type="submit" variant="primary" disabled={!complete || busy}>{submitLabel}</Button>
      </div>
    {/if}
  </form>

  <CliLine command={cli} disabled={!complete} />
</section>

<style>
  .add {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-16);
    border: 1px solid var(--line);
    border-radius: var(--radius-12);
    background: var(--surface);
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
    font-weight: 600;
  }
  .sources {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .source {
    display: grid;
    grid-template-columns: auto 8rem 1fr;
    align-items: baseline;
    gap: var(--space-8);
    padding: var(--space-6) var(--space-8);
    border-radius: var(--radius-8);
    cursor: pointer;
  }
  .source.on {
    background: var(--accent-weak);
  }
  .label {
    font-weight: 600;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  fieldset {
    display: flex;
    flex-direction: column;
    gap: var(--space-10);
    margin: 0;
    padding: 0;
    border: 0;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    font-size: var(--fs-13);
  }
  .field input {
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    color: var(--ink);
  }
  small {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  small.bad {
    color: var(--danger-ink);
  }
  .stale {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-8);
  }
  .stale p {
    margin: 0;
  }
</style>
