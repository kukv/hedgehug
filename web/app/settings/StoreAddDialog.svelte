<script lang="ts" module>
  // The name a URL suggests: its last part, without a trailing slash and without .git.
  export function suggestName(url: string): string {
    const last = url
      .trim()
      .replace(/\/+$/, "")
      .replace(/\.git$/, "");
    return last.split(/[/:]/).pop() ?? "";
  }

  // One URL to a line; blank lines are dropped.
  export function splitUrls(text: string): string[] {
    return text
      .split("\n")
      .map((l) => l.trim())
      .filter((l) => l !== "");
  }
</script>

<script lang="ts">
  import { ApiError, unwrap } from "../../core/api/client";
  import { cliStoreAdd, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Mounts a Store that already exists on a remote: the first URL is cloned, the others are mirrors.
  type Props = { open: boolean; onclose: () => void };
  let { open, onclose }: Props = $props();

  const c = core();
  const uid = $props.id();
  const accounts = useSubscribe(c.accounts, () => c.accounts.accounts());

  let name = $state("");
  let urls = $state("");
  let account = $state("");
  // The name the URL last suggested; the name follows the URL only while it is this (or empty).
  let suggested = "";
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);
  // The accounts the server named when it asked for a choice (400 choice.required).
  let candidates = $state<string[] | null>(null);

  const list = $derived(splitUrls(urls));
  const nameError = $derived(name.trim() === "" ? "名前を入れてください" : null);
  const urlError = $derived(list.length === 0 ? "URL を入れてください" : null);
  const choices = $derived(
    candidates ?? (accounts.current.length >= 2 ? accounts.current.map((a) => a.name) : null),
  );
  const cli = $derived(
    cliStoreAdd({
      name: name.trim() || new Placeholder("名前"),
      remotes: list.length > 0 ? list : [new Placeholder("URL")],
      account: account || undefined,
    }),
  );

  // The accounts are read when the dialog opens: the server asks GitHub for them, so nothing else does.
  $effect(() => {
    if (open) void c.accounts.load();
  });

  function onurl(value: string): void {
    urls = value;
    const next = suggestName(splitUrls(value)[0] ?? "");
    if (name === "" || name === suggested) name = next;
    suggested = next;
  }

  function reset(): void {
    name = "";
    urls = "";
    account = "";
    suggested = "";
    touched = false;
    failure = null;
    candidates = null;
  }

  function close(): void {
    if (busy) return;
    failure = null;
    onclose();
  }

  async function submit(): Promise<void> {
    touched = true;
    if (busy || nameError || urlError) return;
    busy = true;
    failure = null;
    try {
      const r = await unwrap(
        c.api.POST("/api/stores/add", {
          body: { name: name.trim(), remotes: list, account: account || undefined },
        }),
      );
      // The Store is mounted; a failed reload must not read as a failed add (the SSE resync catches up).
      await c.stores.reload().catch(() => undefined);
      toast({ message: `Store「${r.name}」を追加しました` });
      reset();
      onclose();
    } catch (e) {
      if (e instanceof ApiError && e.type === "choice.required" && e.args.what === "--account") {
        const names = e.args.candidates;
        candidates = Array.isArray(names) ? names.map(String) : null;
      }
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent): void {
    if (e.key === "Enter" && !e.isComposing && (e.target as HTMLElement).tagName === "INPUT") {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog
  {open}
  title="URL で追加"
  width={620}
  {cli}
  cliDisabled={!!(nameError || urlError)}
  onclose={close}
>
  <div class="form" role="presentation" {onkeydown}>
    <div class="field">
      <label for="{uid}-url">URL（1行に1つ）</label>
      <textarea
        id="{uid}-url"
        class="mono"
        rows="3"
        value={urls}
        placeholder="git@github.com:org/hh-tasks.git"
        oninput={(e) => onurl(e.currentTarget.value)}
        aria-invalid={touched && urlError ? "true" : undefined}
        aria-describedby="{uid}-url-msg"
      ></textarea>
      <span id="{uid}-url-msg" class="msg" class:bad={touched && urlError}>
        {touched && urlError
          ? urlError
          : "1つ目から取り込みます。2つ目以降はミラーとして書き込み先に加えます"}
      </span>
    </div>

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
        {touched && nameError
          ? nameError
          : "この Store を呼ぶ名前です。URL の最後の部分を提案します"}
      </span>
    </div>

    {#if choices}
      <div class="field">
        <label for="{uid}-account">アカウント</label>
        <select id="{uid}-account" bind:value={account}>
          <option value="">選ばない</option>
          {#each choices as a (a)}
            <option value={a}>{a}</option>
          {/each}
        </select>
        <span class="msg">取り込みと書き込みに使うアカウントです</span>
      </div>
    {/if}

    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={submit}>
      {busy ? "追加中…" : "追加する"}
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
  input,
  select {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  textarea {
    padding: var(--space-10) var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    resize: vertical;
  }
  [aria-invalid="true"] {
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
</style>
