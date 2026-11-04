<script lang="ts">
  import { cliViewSave, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { saveBodyOf, type ViewDraft } from "../../core/views";
  import { core } from "../context";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { sameViewName, summaryOf, treeOrderProblem } from "./draft";

  // store is the Store the dialog starts with (the one the screen is about); without it, the default.
  let {
    draft,
    store: asked,
    onclose,
  }: { draft: ViewDraft; store?: string; onclose: () => void } = $props();

  const { stores, views } = core();
  const uid = $props.id();
  const dir = useSubscribe(stores, () => stores.stores());
  const all = useSubscribe(views, () => views.list());

  // Only a Store that can be written is a place to save to.
  const writable = $derived(dir.current.filter((s) => stores.tone(s.name) !== "readonly"));
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
  // Whether the views of the Store are known: a name is checked against them.
  const known = useSubscribe(views, () => ({
    loaded: views.loaded(),
    failed: views.failed(store),
  }));
  const storeProblem = $derived.by(() => {
    if (dir.current.length > 0 && store === "") {
      return "書き込める Store がありません（すべて閲覧のみです）";
    }
    if (store !== "" && known.current.failed) {
      return `${store} のビューを読み込めないため、同じ名前のビューがあるか確かめられません`;
    }
    return null;
  });
  const loading = $derived(!known.current.loaded);

  let name = $state("");
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  // PUT would replace a view of that name, so a name in use is not saved here (the view's own
  // screen updates it).
  const taken = $derived(
    all.current.find((e) => e.store === store && sameViewName(e.view.name, name)),
  );
  const orderProblem = $derived(treeOrderProblem(draft));
  const nameProblem = $derived(
    taken
      ? `${taken.view.name} というビューが ${store} にあります。置き換えるには、そのビューの画面の「ビューを更新」を使います`
      : null,
  );
  const invalid = $derived(
    store === "" ||
      name === "" ||
      loading ||
      storeProblem !== null ||
      nameProblem !== null ||
      orderProblem !== null,
  );
  // An empty field is not scolded before the first try.
  const nameShown = $derived(name === "" && !touched ? null : nameProblem);

  const cli = $derived(
    cliViewSave({
      name: name === "" ? new Placeholder("名前") : name,
      store,
      body: saveBodyOf(draft),
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
      const saved = await views.save(store, name, saveBodyOf(draft));
      toast({ message: `ビュー ${saved.name} を ${store} に保存しました` });
      router.go(router.href({ name: "view", store, view: saved.name }));
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

<Dialog open title="ビューを保存" width={560} {cli} cliDisabled={invalid} onclose={close}>
  <div class="form" role="presentation" {onkeydown}>
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
        {nameShown ?? '1〜64 文字。/ \\ : * ? " < > | は使えません'}
      </span>
    </div>

    {#if dir.current.length >= 2}
      <div class="field" role="radiogroup" aria-label="保存先">
        <span class="lbl">保存先</span>
        <div class="stores">
          {#each dir.current as s (s.name)}
            {@const ro = stores.tone(s.name) === "readonly"}
            <!-- biome-ignore lint/a11y/noLabelWithoutControl: the badge is the text of the label -->
            <label class="choice" class:on={s.name === store} class:off={ro}>
              <input
                type="radio"
                name="{uid}-store"
                value={s.name}
                checked={s.name === store}
                disabled={ro}
                onchange={() => (picked = s.name)}
              >
              <StoreBadge store={s.name} tone={stores.tone(s.name)} />
              {#if ro}
                <span class="note">閲覧のみ</span>
              {/if}
            </label>
          {/each}
        </div>
        <span class="msg">{store} の views/{name === "" ? "<名前>" : name}.yaml に保存します</span>
      </div>
    {:else}
      <p class="msg">{store} の views/{name === "" ? "<名前>" : name}.yaml に保存します</p>
    {/if}

    <dl class="summary">
      <dt>設定</dt>
      <dd>{summaryOf(draft)}</dd>
      <dt>クエリ</dt>
      <dd class="mono">{draft.query === "" ? "（すべてのタスク）" : draft.query}</dd>
    </dl>

    {#if storeProblem}
      <p class="problem" role="alert">{storeProblem}</p>
    {:else if loading}
      <p class="msg">ビューを読み込んでいます</p>
    {/if}
    {#if orderProblem}
      <p class="problem" role="alert">{orderProblem}</p>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {busy ? "保存中…" : "保存"}
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
    cursor: not-allowed;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
  .summary {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--space-6) var(--space-12);
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--sunken);
    font-size: var(--fs-13);
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .problem {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--warning-weak);
    font-size: var(--fs-13);
    line-height: 1.6;
  }
</style>
