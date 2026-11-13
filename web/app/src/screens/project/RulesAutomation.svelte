<script lang="ts">
  import {
    type AutomationRuleInput,
    deleteRuleBody,
    deleteRuleTitle,
    hasRules,
    type ProjectView,
    projectRuleRows,
    Refused,
    readOnlyNote,
    ruleCreated,
    ruleDeleted,
    rulesEmpty,
    rulesOf,
    switchOnRefusal,
    usedStatusRows,
    type WorkspaceAutomation,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, IconButton, Switch } from "@hedgehug/ui";
  import { untrack } from "svelte";
  import { projectsPort, rulesEditor } from "../../projectEditor";
  import { toasts, workspacesPort } from "../../wire";
  import RuleDialog from "../workspace-settings/RuleDialog.svelte";
  import ResetRulesButton from "./ResetRulesButton.svelte";

  // 「自動化」の区画(03「プロジェクトの自動化」)。ルールは、このプロジェクトのタブを開いているあいだだけ読む。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const disabled = $derived(view.store.readOnly);
  const title = $derived(disabled ? readOnlyNote : undefined);

  let automation = $state<WorkspaceAutomation | null>(null);
  let editing = $state<string | "new" | null>(null);
  let deleting = $state<string | null>(null);

  async function load() {
    automation = await workspacesPort.automation(p.storeId, p.workspaceId);
  }
  const workspaceKey = $derived(`${p.storeId}/${p.workspaceId}`);
  $effect(() => {
    void workspaceKey;
    untrack(() => void load());
  });

  const rows = $derived(
    automation ? projectRuleRows(automation, view.settings, view.members, p.id) : [],
  );
  const own = $derived(rows.filter((r) => r.own).length);
  const empty = $derived(!hasRules(rulesOf(view.settings, p.id), "automation") && own === 0);
  const extra = $derived(
    own > 0 ? `このプロジェクトだけのルール ${own} 件も削除します。` : undefined,
  );
  const statuses = $derived(
    usedStatusRows(view.settings, p.id, view.items)
      .rows.filter((r) => r.checked)
      .map((r) => ({ value: r.id, label: r.name })),
  );
  const editingRule = $derived(automation?.rules.find((r) => r.id === editing) ?? null);
  const deletingName = $derived(rows.find((r) => r.id === deleting)?.name ?? "");

  // 操作のあと、画面の読み直し(rulesEditor)とルールの読み直しをして、スイッチを保存済みの値に戻す。
  async function run(op: () => Promise<unknown>, ok?: string) {
    await rulesEditor.run(op, ok);
    await load();
  }

  function toggle(id: string, own: boolean, on: boolean) {
    const rule = automation?.rules.find((r) => r.id === id);
    const refusal = own && on && rule ? switchOnRefusal(rule) : null;
    if (refusal) {
      toasts.show(refusal, { error: true });
      return;
    }
    void run(() =>
      own
        ? workspacesPort.setRuleEnabled(p.storeId, p.workspaceId, id, on)
        : projectsPort.setRuleRuns(p.storeId, p.id, id, on),
    );
  }

  // ダイアログの保存。断られたら、その文言を返してダイアログの中に出す。
  async function save(input: AutomationRuleInput): Promise<string | null> {
    const id = editingRule?.id;
    try {
      if (id) await workspacesPort.updateRule(p.storeId, p.workspaceId, id, input);
      else await workspacesPort.createRule(p.storeId, p.workspaceId, { ...input, scope: p.id });
    } catch (e) {
      if (e instanceof Refused) return e.message;
      throw e;
    }
    await load();
    if (!id) toasts.show(ruleCreated);
    return null;
  }

  function remove() {
    const id = deleting;
    deleting = null;
    if (id === null) return;
    void run(() => workspacesPort.deleteRule(p.storeId, p.workspaceId, id), ruleDeleted);
  }
</script>

<section class="sec">
  <h2 id="rules-automation" tabindex="-1">自動化</h2>
  <div class="rows">
    {#each rows as row (row.id)}
      <div class="row">
        <Switch
          label={`${row.name} を${row.own ? "有効にする" : "ここで動かす"}`}
          checked={row.enabled}
          disabled={disabled || row.locked}
          {title}
          onchange={(on) => toggle(row.id, row.own, on)}
        />
        <span class="summary">{row.summary}</span>
        <strong>{row.name}</strong>
        <span class="summary">{row.origin}{row.note ? `(${row.note})` : ""}</span>
        <span class="grow"></span>
        {#if row.own}
          <Button {disabled} {title} onclick={() => (editing = row.id)}>編集</Button>
          <IconButton label="削除" size="sm" {disabled} {title} onclick={() => (deleting = row.id)}
            >🗑</IconButton
          >
        {/if}
      </div>
    {:else}
      <p class="summary">{rulesEmpty}</p>
    {/each}
  </div>
  <div>
    <Button {disabled} {title} onclick={() => (editing = "new")}
      >このプロジェクトだけのルールを追加</Button
    >
  </div>
  <ResetRulesButton {view} section="automation" label="自動化" {empty} {extra} />
</section>

{#if editing !== null && automation}
  <RuleDialog
    settings={view.settings}
    members={view.members}
    readOnly={disabled}
    rules={automation.rules}
    rule={editingRule}
    {statuses}
    onsave={save}
    onclose={() => (editing = null)}
  />
{/if}

{#if deleting !== null}
  <ConfirmDialog
    title={deleteRuleTitle}
    lines={[deleteRuleBody(deletingName)]}
    ok="削除"
    danger
    onok={remove}
    oncancel={() => (deleting = null)}
  />
{/if}

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h2 {
    font-size: 14px;
    margin: 0;
  }
  .rows {
    display: flex;
    flex-direction: column;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
    flex-wrap: wrap;
  }
  .summary {
    font-size: 12.5px;
    color: var(--muted);
    margin: 0;
  }
  .grow {
    flex: 1;
  }
</style>
