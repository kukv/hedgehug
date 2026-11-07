<script lang="ts">
  import { untrack } from "svelte";
  import { cliWorkspaceRename, cliWorkspaceRm, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { nameProblem, prefixDay, removeBlocker } from "../../core/workspace";
  import { core } from "../context";
  import { openPlanDialog } from "../planning/dialogs-state.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import { copyText } from "../ui/clipboard";
  import FormError from "../ui/FormError.svelte";
  import { openPrefixChange, openWorkspaceDelete } from "./dialogs-state.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const { api, stores } = core();
  const uid = $props.id();
  const all = useSubscribe(stores, () => stores.workspaces());
  // The settings of a Store that cannot be written are read, not saved (細部 35).
  const offSentence = useReadOnlySentence(stores, () => entry.store);
  const off = $derived(offSentence.current);

  // The name in the field. It follows the saved name when that changes (a rename from the CLI, or the
  // save below), and is left alone while anything else about the workspace changes.
  const saved = $derived(entry.name);
  let draft = $state(untrack(() => entry.name));
  $effect(() => {
    draft = saved;
  });

  let busy = $state(false);
  let failure = $state<string | null>(null);

  const unchanged = $derived(draft === saved);
  const problem = $derived(
    unchanged ? null : nameProblem(draft, entry.store, all.current, entry.ulid),
  );
  const count = $derived(entry.tasks + entry.deleted);
  // The day the prefix was last set, as a date (this year without the year).
  const prefixOn = $derived(prefixDay(entry.prefixSet, new Date()));
  const blocker = $derived(removeBlocker(entry));

  const renameCli = $derived(
    cliWorkspaceRename({
      workspace: saved,
      name: unchanged || problem ? new Placeholder("新しい名前") : draft,
      store: entry.store,
    }),
  );

  async function save() {
    if (busy || unchanged || problem !== null || off !== null) return;
    busy = true;
    failure = null;
    const from = saved;
    try {
      const r = await api.updateWorkspace(entry.ulid, { name: draft }, { store: entry.store });
      await stores.reload().catch(() => undefined);
      toast({ message: `ワークスペースの名前を ${from} から ${r.workspace.name} に変えました` });
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229) {
      e.preventDefault();
      void save();
    }
  }

  async function copyUlid() {
    toast({
      message: (await copyText(entry.ulid)) ? "ULID をコピーしました" : "コピーできませんでした",
    });
  }
</script>

<div class="settings">
  <section class="block" aria-labelledby="{uid}-name-h">
    <h2 id="{uid}-name-h">名前</h2>
    <div class="row">
      <input
        id="{uid}-name"
        type="text"
        autocomplete="off"
        aria-label="名前"
        readonly={off !== null}
        title={off ?? undefined}
        value={draft}
        oninput={(e) => (draft = e.currentTarget.value)}
        {onkeydown}
        aria-invalid={problem ? "true" : undefined}
        aria-describedby="{uid}-name-msg"
      >
      <Button
        variant="primary"
        disabled={busy || unchanged || problem !== null || off !== null}
        why={off ?? undefined}
        onclick={() => void save()}
      >
        保存
      </Button>
    </div>
    <p id="{uid}-name-msg" class="msg" class:bad={problem}>
      {#if off}
        {off}
      {:else if problem}
        {problem}
      {:else if unchanged}
        タスクの ID は変わりません
      {:else}
        名前だけが変わります。接頭辞 {entry.prefix} とタスクの ID はそのままです
      {/if}
    </p>
    {#if failure}
      <FormError message={failure} />
    {/if}
    <CliLine command={renameCli} disabled={unchanged || problem !== null} />
  </section>

  <section class="block" aria-labelledby="{uid}-plan-h">
    <h2 id="{uid}-plan-h">計画</h2>
    <div class="row">
      <Button
        disabled={off !== null}
        why={off ?? undefined}
        onclick={() => openPlanDialog({ kind: "sprint-add", entry })}
      >
        スプリントを作成…
      </Button>
      <Button
        disabled={off !== null}
        why={off ?? undefined}
        onclick={() => openPlanDialog({ kind: "release-add", entry })}
      >
        リリースを作成…
      </Button>
    </div>
    <p class="msg">
      スプリント {entry.sprints} 件 · リリース {entry.releases} 件。1
      件以上あると、バックログ・スプリント・リリースのタブが出ます
    </p>
  </section>

  <section class="block" aria-labelledby="{uid}-prefix-h">
    <h2 id="{uid}-prefix-h">ID の接頭辞</h2>
    <div class="row">
      <span class="prefix mono">{entry.prefix}</span>
      <Button disabled={off !== null} why={off ?? undefined} onclick={() => openPrefixChange(entry)}
        >接頭辞を変更…</Button
      >
    </div>
    <p class="msg">
      このワークスペースのタスクは {entry.prefix}-1, {entry.prefix}-2 … と振ります ·
      最後に変えたのは {prefixOn}
    </p>
    <p class="msg">
      変えると、配下の {count} 件（削除済みを含む）の ID を新しい接頭辞で振り直します（1
      コミット）。古い ID でも引き続き検索できます。
    </p>
  </section>

  <section class="block" aria-labelledby="{uid}-ulid-h">
    <h2 id="{uid}-ulid-h">ULID</h2>
    <div class="row">
      <span class="mono">{entry.ulid}</span>
      <button type="button" class="copy" aria-label="ULID をコピー" onclick={copyUlid}>
        コピー
      </button>
    </div>
    <p class="msg">
      名前や接頭辞を変えても変わりません。CLI では名前・接頭辞・ULID のどれでも指せます
    </p>
  </section>

  <section class="block" aria-labelledby="{uid}-rm-h">
    <h2 id="{uid}-rm-h">ワークスペースを削除</h2>
    {#if blocker}
      <p class="msg" role="note">{blocker}</p>
    {:else}
      <p class="msg">タスクがないので削除できます。削除すると接頭辞 {entry.prefix} は空きます。</p>
    {/if}
    <div class="row">
      <Button
        variant="danger"
        disabled={blocker !== null || off !== null}
        why={off ?? undefined}
        onclick={() => openWorkspaceDelete(entry)}
      >
        削除
      </Button>
    </div>
    <CliLine
      command={cliWorkspaceRm({ workspace: entry.name, store: entry.store })}
      disabled={blocker !== null}
    />
  </section>
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    max-width: 760px;
    padding: 0 var(--gutter);
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-20) 0;
    border-bottom: 1px solid var(--line);
  }
  .block:last-child {
    border-bottom: 0;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .row {
    display: flex;
    gap: var(--space-8);
    align-items: center;
  }
  input {
    flex: 1;
    max-width: 360px;
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .prefix {
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .bad {
    color: var(--danger);
  }
  .copy {
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
</style>
