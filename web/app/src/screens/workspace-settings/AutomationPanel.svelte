<script lang="ts">
  import {
    type AutomationRuleInput,
    automationHelp,
    deleteRuleBody,
    deleteRuleTitle,
    readOnlyNote,
    recentEmpty,
    recentRows,
    ruleCreated,
    ruleDeleted,
    ruleRows,
    rulesEmpty,
    type SettingsState,
    switchOnRefusal,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, IconButton, Switch } from "@hedgehug/ui";
  import { toasts, workspaceSettings } from "../../wire";
  import RuleDialog from "./RuleDialog.svelte";

  // 自動化のタブ(03「自動化」)。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  let editing = $state<string | "new" | null>(null);
  let deleting = $state<string | null>(null);

  const automation = $derived(s.automation);
  const rows = $derived(automation ? ruleRows(automation, s.detail.settings, s.members) : []);
  const recent = $derived(automation ? recentRows(automation, new Date()) : []);
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const editingRule = $derived(automation?.rules.find((r) => r.id === editing) ?? null);
  const deletingName = $derived(rows.find((r) => r.id === deleting)?.name ?? "");

  function toggle(id: string, on: boolean) {
    const rule = automation?.rules.find((r) => r.id === id);
    const refusal = on && rule ? switchOnRefusal(rule) : null;
    if (refusal) {
      toasts.show(refusal, { error: true });
      return;
    }
    void workspaceSettings.run((p, st, w) => p.setRuleEnabled(st, w, id, on));
  }

  // ダイアログの保存。断られたら、その文言を返してダイアログの中に出す。
  async function save(input: AutomationRuleInput): Promise<string | null> {
    const id = editingRule?.id;
    let refusal: string | null = null;
    const ok = await workspaceSettings.run(
      (p, st, w) => (id ? p.updateRule(st, w, id, input) : p.createRule(st, w, input)),
      id ? undefined : ruleCreated,
      (message) => (refusal = message),
    );
    return ok ? null : (refusal ?? "");
  }

  function remove() {
    const id = deleting;
    deleting = null;
    if (id === null) return;
    void workspaceSettings.run((p, st, w) => p.deleteRule(st, w, id), ruleDeleted);
  }
</script>

{#if automation}
  <div class="panel">
    <p class="help">{automationHelp}</p>
    <div class="rows">
      {#each rows as row (row.id)}
        <div class="row">
          <Switch
            label={`${row.name} を有効にする`}
            checked={row.enabled}
            disabled={s.readOnly}
            {title}
            onchange={(on) => toggle(row.id, on)}
          />
          <span class="name">{row.name}</span>
          <span class="summary">{row.summary}</span>
          <span class="grow"></span>
          <Button onclick={() => (editing = row.id)}>編集</Button>
          <IconButton
            label="削除"
            size="sm"
            disabled={s.readOnly}
            {title}
            onclick={() => (deleting = row.id)}
            >🗑</IconButton
          >
        </div>
      {:else}
        <p class="empty">{rulesEmpty}</p>
      {/each}
    </div>
    <div>
      <Button disabled={s.readOnly} {title} onclick={() => (editing = "new")}>ルールを作成</Button>
    </div>
    <section aria-label="最近の実行">
      <h3>最近の実行</h3>
      {#each recent as r, i (i)}
        <div class="run">
          <span class="time">{r.time}</span>
          <span class="id">{r.display}</span>
          <span>{r.text}</span>
        </div>
      {:else}
        <p class="empty">{recentEmpty}</p>
      {/each}
    </section>
  </div>
{/if}

{#if editing !== null && automation}
  <RuleDialog
    settings={s.detail.settings}
    members={s.members}
    readOnly={s.readOnly}
    rules={automation.rules}
    rule={editingRule}
    statuses={s.detail.settings.statuses.map((v) => ({ value: v.id, label: v.name }))}
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
  .panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 720px;
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
  .name {
    font-weight: 600;
  }
  .summary,
  .empty,
  .help,
  .time {
    font-size: 12.5px;
    color: var(--muted);
  }
  .empty,
  .help {
    margin: 0;
  }
  .grow {
    flex: 1;
  }
  h3 {
    font-size: 13px;
    margin: 0 0 6px;
  }
  .run {
    display: flex;
    gap: 8px;
    font-size: 12.5px;
    padding: 3px 0;
  }
  .id {
    font-family: var(--mono);
  }
</style>
