<script lang="ts">
  import {
    type AutomationAction,
    type AutomationRule,
    type AutomationRuleInput,
    type AutomationTrigger,
    actionOptions,
    allMergedLabel,
    newRuleForm,
    readOnlyNote,
    ruleCreated,
    ruleForm,
    ruleFormRefusal,
    type SettingsState,
    triggerOptions,
    valueOptions,
    withAction,
    withTrigger,
  } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input, Select } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";

  // ルールの作成と編集のダイアログ(03「きっかけとすること」)。断りはダイアログの中に赤字で出す。
  interface Props {
    state: SettingsState;
    rules: AutomationRule[];
    rule: AutomationRule | null; // null は新規
    onclose: () => void;
  }

  let { state: s, rules, rule, onclose }: Props = $props();

  const settings = $derived(s.detail.settings);
  const start = (): AutomationRuleInput => (rule ? ruleForm(rule) : newRuleForm(s.detail.settings));
  let form = $state(start());
  let error = $state("");
  const disabled = $derived(s.readOnly);
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const statusOptions = $derived(settings.statuses.map((v) => ({ value: v.id, label: v.name })));

  async function save() {
    const refusal = ruleFormRefusal(form, rules, rule?.id ?? "");
    if (refusal) {
      error = refusal;
      return;
    }
    const input = { ...form, name: form.name.trim() };
    const ok = await workspaceSettings.run(
      (p, st, w) => (rule ? p.updateRule(st, w, rule.id, input) : p.createRule(st, w, input)),
      rule ? undefined : ruleCreated,
      (message) => (error = message),
    );
    if (ok) onclose();
  }
</script>

<Dialog open title={rule ? "ルールの編集" : "ルールを作成"} {onclose}>
  <div class="form">
    <Input label="名前" bind:value={form.name} {disabled} {title} focusOnMount={!disabled} />
    <Select
      label="きっかけ"
      value={form.trigger}
      options={triggerOptions}
      {disabled}
      {title}
      onchange={(v) => (form = withTrigger(form, v as AutomationTrigger, settings))}
    />
    {#if form.trigger === "status"}
      <Select
        label="ステータス"
        value={form.triggerStatus}
        options={statusOptions}
        {disabled}
        {title}
        onchange={(v) => (form = { ...form, triggerStatus: v })}
      />
    {/if}
    {#if form.trigger === "pr_merged"}
      <Checkbox label={allMergedLabel} bind:checked={form.allMerged} {disabled} {title} />
    {/if}
    <Select
      label="すること"
      value={form.action}
      options={actionOptions}
      {disabled}
      {title}
      onchange={(v) => (form = withAction(form, v as AutomationAction, settings))}
    />
    <Select
      label="値"
      value={form.value}
      options={valueOptions(form, settings, s.members)}
      {disabled}
      {title}
      onchange={(v) => (form = { ...form, value: v })}
    />
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>{disabled ? "閉じる" : "キャンセル"}</Button>
    <Button variant="primary" {disabled} {title} onclick={save}>保存</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
