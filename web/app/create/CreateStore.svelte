<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliStoreInit, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { hrefOf, router } from "../router";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";

  type Props = { open: boolean; onclose: () => void };
  let { open, onclose }: Props = $props();

  const { api, stores } = core();
  const uid = $props.id();

  // The same rule as ValidIDPrefix in internal/model: 2 to 10 upper-case letters and digits, starting with a letter.
  const PREFIX = /^[A-Z][A-Z0-9]{1,9}$/;

  let name = $state("");
  let prefix = $state("");
  let workspace = $state("");
  let remote = $state("");
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const nameError = $derived(name.trim() === "" ? "名前を入れてください" : null);
  const prefixError = $derived(
    PREFIX.test(prefix) ? null : "接頭辞は英大文字と数字の 2〜10 文字で、英字から始めてください",
  );
  const remotes = $derived(remote.trim() ? [remote.trim()] : undefined);
  const cli = $derived(
    cliStoreInit({
      name: name.trim() || new Placeholder("名前"),
      prefix: prefix || new Placeholder("接頭辞"),
      workspace: workspace.trim() || undefined,
      remotes,
    }),
  );

  function reset() {
    name = "";
    prefix = "";
    workspace = "";
    remote = "";
    touched = false;
    failure = null;
  }

  // While a create is running the dialog stays: closing it then would let the success navigate away unasked.
  function close() {
    if (busy) return;
    failure = null;
    onclose();
  }

  async function submit() {
    touched = true;
    if (busy || nameError || prefixError) return;
    busy = true;
    failure = null;
    try {
      const r = await unwrap(
        api.POST("/api/stores", {
          body: { name: name.trim(), prefix, workspace: workspace.trim() || undefined, remotes },
        }),
      );
      // The store exists now; a failed reload must not read as a failed create (the SSE resync catches up).
      await stores.reload().catch(() => undefined);
      toast({ message: `Store「${r.name}」を作りました` });
      reset();
      onclose();
      // The tree, where the first task is added.
      router.go(hrefOf({ name: "workspace", store: r.name, ulid: r.workspace.ulid, tab: "tree" }));
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && (e.target as HTMLElement).tagName === "INPUT") {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog
  {open}
  title="Store を作成"
  width={620}
  {cli}
  cliDisabled={!!(nameError || prefixError)}
  onclose={close}
>
  <div class="form" role="presentation" {onkeydown}>
    <div class="field">
      <label for="{uid}-name">名前</label>
      <input
        id="{uid}-name"
        type="text"
        value={name}
        oninput={(e) => (name = e.currentTarget.value)}
        aria-invalid={touched && nameError ? "true" : undefined}
        aria-describedby="{uid}-name-msg"
      >
      <span id="{uid}-name-msg" class="msg" class:bad={touched && nameError}>
        {touched && nameError ? nameError : "この Store を呼ぶ名前です"}
      </span>
    </div>

    <div class="field">
      <label for="{uid}-prefix">ID の接頭辞</label>
      <input
        id="{uid}-prefix"
        class="mono"
        type="text"
        value={prefix}
        oninput={(e) => (prefix = e.currentTarget.value.toUpperCase())}
        aria-invalid={touched && prefixError ? "true" : undefined}
        aria-describedby="{uid}-prefix-msg"
      >
      <span id="{uid}-prefix-msg" class="msg" class:bad={touched && prefixError}>
        {touched && prefixError
          ? prefixError
          : "タスクの ID が ME-1 のようになります。最初のワークスペースの接頭辞です"}
      </span>
    </div>

    <div class="field">
      <label for="{uid}-ws">最初のワークスペース（任意）</label>
      <input
        id="{uid}-ws"
        type="text"
        value={workspace}
        placeholder={name.trim()}
        oninput={(e) => (workspace = e.currentTarget.value)}
      >
      <span class="msg">空なら Store と同じ名前にします</span>
    </div>

    <div class="field">
      <label for="{uid}-remote">remote（任意）</label>
      <input
        id="{uid}-remote"
        class="mono"
        type="text"
        value={remote}
        placeholder="git@github.com:org/hh-tasks.git"
        oninput={(e) => (remote = e.currentTarget.value)}
      >
      <span class="msg">あれば最初の push までします。なくてもこのマシンだけで使えます</span>
    </div>

    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={submit}>
      {busy ? "作成中…" : "Store を作る"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  label {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  input {
    min-height: 40px;
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
</style>
