<script lang="ts">
  import { onDestroy } from "svelte";
  import { ApiError, type Schemas } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Button from "../ui/Button.svelte";
  import { copyText } from "../ui/clipboard";
  import FormError from "../ui/FormError.svelte";

  // The code of a Device Flow that is running: where to enter it, how long it lasts, and the wait
  // for the server to say (auth.device) that GitHub approved it. The starting and the cancelling of
  // the flow are the parent's.
  type Props = {
    flow: Schemas.DeviceStartBody;
    // a flow that another screen started
    resumed?: boolean;
    ondone: (account: string) => void;
    onretry: () => void;
  };
  let { flow, resumed = false, ondone, onretry }: Props = $props();

  const { conn } = core();

  let now = $state(Date.now());
  let copied = $state(false);
  let failure = $state<string | null>(null);
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;

  const left = $derived(Math.max(0, Math.ceil((Date.parse(flow.expires_at) - now) / 1000)));
  const expired = $derived(left === 0);
  const clock = $derived(
    `${String(Math.floor(left / 60)).padStart(2, "0")}:${String(left % 60).padStart(2, "0")}`,
  );

  // The link is drawn only for a web address.
  const link = $derived.by(() => {
    try {
      const u = new URL(flow.verification_uri);
      if (u.protocol !== "https:" && u.protocol !== "http:") return null;
      return { href: u.href, text: `${u.host}${u.pathname.replace(/\/$/, "")} を開く` };
    } catch {
      return null;
    }
  });

  $effect(() => {
    now = Date.now();
    const tick = setInterval(() => {
      now = Date.now();
    }, 1000);
    return () => clearInterval(tick);
  });

  $effect(() => {
    const id = flow.id;
    failure = null;
    return conn.on("auth.device", (e) => {
      if (e.id !== id) return;
      if (e.state === "done") {
        ondone(e.account ?? "");
        return;
      }
      failure = errorMessage(
        new ApiError({
          status: 0,
          type: e.code ?? "auth.device_failed",
          detail: e.message ?? "",
        }),
      );
    });
  });

  async function copy() {
    if (!(await copyText(flow.user_code))) return;
    copied = true;
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => {
      copied = false;
    }, 1500);
  }

  onDestroy(() => clearTimeout(copiedTimer));
</script>

<div class="flow">
  {#if resumed}
    <p class="note">別の画面で始めたログインを続けます</p>
  {/if}
  {#if failure !== null}
    <FormError message={failure} />
    <div><Button onclick={onretry}>やり直す</Button></div>
  {:else if expired}
    <FormError message="コードの期限が切れました" />
    <div><Button onclick={onretry}>やり直す</Button></div>
  {:else}
    <p class="lead">このコードを GitHub で入力してください</p>
    <div class="code">
      <code class="user-code">{flow.user_code}</code>
      <Button size="sm" onclick={copy}>{copied ? "コピーしました" : "コピー"}</Button>
    </div>
    {#if link}
      <a href={link.href} target="_blank" rel="noopener noreferrer">{link.text}</a>
    {/if}
    <p class="wait">承認を待っています · コードの期限まで {clock}</p>
  {/if}
</div>

<style>
  .flow {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-10);
  }
  .lead,
  .note,
  .wait {
    margin: 0;
  }
  .note,
  .wait {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .code {
    display: flex;
    align-items: center;
    gap: var(--space-12);
  }
  .user-code {
    padding: var(--space-8) var(--space-12);
    border-radius: var(--radius-8);
    background: var(--sunken);
    font-family: var(--font-mono);
    font-size: var(--fs-19);
    font-weight: 600;
    letter-spacing: 0.1em;
  }
  a {
    color: var(--accent);
  }
</style>
