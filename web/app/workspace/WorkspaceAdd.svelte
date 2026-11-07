<script lang="ts">
  import { cliWorkspaceAdd, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { nameProblem, prefixProblem } from "../../core/workspace";
  import { core } from "../context";
  import { storeShown } from "../hidden";
  import { useReadOnlyOf } from "../read-only";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";

  // store is the Store the dialog starts with (the settings page of that Store opened it).
  let { store: asked, onclose }: { store?: string; onclose: () => void } = $props();

  const { api, stores } = core();
  const uid = $props.id();
  const dir = useSubscribe(stores, () => ({
    items: stores.stores(),
    workspaces: stores.workspaces(),
  }));

  const roOf = useReadOnlyOf(stores);

  // A hidden Store is not offered (細部 93).
  const items = $derived(dir.current.items.filter((s) => storeShown(stores, s.name)));
  // Only a Store that can be written is a place to add to (細部 99).
  const writable = $derived(items.filter((s) => roOf.current(s.name) === null));
  // The Store picked here, else the one asked for, else the default one, else the first (of those
  // that can be written).
  let picked = $state("");
  const store = $derived(
    writable.find((s) => s.name === picked)?.name ??
      writable.find((s) => s.name === asked)?.name ??
      writable.find((s) => s.default)?.name ??
      writable[0]?.name ??
      "",
  );
  const noWritable = $derived(items.length > 0 && writable.length === 0);

  let name = $state("");
  let prefix = $state("");
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const nameError = $derived(nameProblem(name, store, dir.current.workspaces));
  const prefixError = $derived(prefixProblem(prefix, store, dir.current.workspaces));
  const invalid = $derived(store === "" || nameError !== null || prefixError !== null);
  // An empty field is not scolded before the first try.
  const nameShown = $derived(name === "" && !touched ? null : nameError);
  const prefixShown = $derived(prefix === "" && !touched ? null : prefixError);

  const cli = $derived(
    cliWorkspaceAdd({
      name: name === "" ? new Placeholder("名前") : name,
      prefix: prefix === "" ? new Placeholder("接頭辞") : prefix,
      store,
    }),
  );

  // While the write runs the dialog stays: closing it then would hide its error.
  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    touched = true;
    if (busy || invalid) return;
    busy = true;
    failure = null;
    try {
      const r = await api.addWorkspace({ name, prefix }, { store });
      // The workspace exists now; a failed reload must not read as a failed add (the SSE catches up).
      await stores.reload().catch(() => undefined);
      toast({
        message: `ワークスペース ${r.workspace.name}（${r.workspace.id_prefix}）を ${store} に追加しました`,
        link: {
          label: "開く",
          href: router.href({
            name: "workspace",
            store,
            ulid: r.workspace.ulid,
            tab: "overview",
          }),
        },
      });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    const t = e.target as HTMLInputElement;
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && t.type === "text") {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog
  open
  title={store === "" ? "ワークスペースを追加" : `${store} にワークスペースを追加`}
  width={560}
  {cli}
  cliDisabled={invalid}
  onclose={close}
>
  <div class="form" role="presentation" {onkeydown}>
    {#if items.length >= 2}
      <div class="field" role="radiogroup" aria-label="Store">
        <span class="lbl">Store</span>
        <div class="stores">
          {#each items as s (s.name)}
            {@const why = roOf.current(s.name)}
            <!-- biome-ignore lint/a11y/noLabelWithoutControl: the badge is the text of the label -->
            <label
              class="choice"
              class:on={s.name === store}
              class:off={why !== null}
              title={why ?? undefined}
            >
              <input
                type="radio"
                name="{uid}-store"
                value={s.name}
                checked={s.name === store}
                disabled={why !== null}
                aria-describedby={why !== null ? `${uid}-${s.name}-off` : undefined}
                onchange={() => (picked = s.name)}
              >
              <StoreBadge store={s.name} tone={stores.tone(s.name)} />
              {#if why !== null}
                <span id="{uid}-{s.name}-off" class="note">閲覧のみ（{why}）</span>
              {/if}
            </label>
          {/each}
        </div>
      </div>
    {/if}

    <div class="field">
      <label class="lbl" for="{uid}-name">名前</label>
      <input
        id="{uid}-name"
        type="text"
        autocomplete="off"
        data-autofocus
        value={name}
        oninput={(e) => (name = e.currentTarget.value)}
        aria-invalid={nameShown ? "true" : undefined}
        aria-describedby="{uid}-name-msg"
      >
      <span id="{uid}-name-msg" class="msg" class:bad={nameShown}>
        {nameShown ?? "小文字の英数字と - _ が使えます。同じ Store の中で同じ名前は使えません"}
      </span>
    </div>

    <div class="field">
      <label class="lbl" for="{uid}-prefix">ID の接頭辞</label>
      <input
        id="{uid}-prefix"
        class="mono"
        type="text"
        autocomplete="off"
        value={prefix}
        oninput={(e) => (prefix = e.currentTarget.value.toUpperCase())}
        aria-invalid={prefixShown ? "true" : undefined}
        aria-describedby="{uid}-prefix-msg"
      >
      <span id="{uid}-prefix-msg" class="msg" class:bad={prefixShown}>
        {prefixShown ??
          "接頭辞は英大文字と数字の 2〜10 文字で、英字から始めてください。同じ Store とマウント中の Store のワークスペースと同じ接頭辞は使えません"}
      </span>
    </div>

    {#if noWritable}
      <FormError message="書き込める Store がありません（すべて閲覧のみです）" />
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {busy ? "追加中…" : "追加"}
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
  .lbl {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  input[type="text"] {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-13);
  }
  .stores {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }
  .choice {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    padding: var(--space-6) var(--space-10);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    cursor: pointer;
  }
  .choice.on {
    border-color: var(--accent);
    background: var(--accent-weak);
  }
  .choice.off {
    color: var(--muted);
    cursor: default;
  }
  .note {
    color: var(--muted);
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
