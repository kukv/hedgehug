<script lang="ts">
  import {
    type AutomationAction,
    type AutomationRule,
    type AutomationRuleInput,
    type AutomationTrigger,
    actionOptions,
    allMergedLabel,
    type Member,
    newRuleForm,
    readOnlyNote,
    ruleForm,
    ruleFormRefusal,
    triggerOptions,
    valueOptions,
    type WorkspaceSettings,
    withAction,
    withTrigger,
  } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input, Select } from "@hedgehug/ui";

  // ルールの作成と編集のダイアログ(03「きっかけとすること」)。断りはダイアログの中に赤字で出す。
  // 保存の中身(どこへ送るか、scope)は呼び出し側の onsave が決め、断りの文言か、成功なら null を返す。
  interface Props {
    settings: WorkspaceSettings;
    members: Member[];
    readOnly: boolean;
    rules: AutomationRule[];
    rule: AutomationRule | null; // null は新規
    statuses: { value: string; label: string }[]; // きっかけとすることのステータスの選択肢
    onsave: (input: AutomationRuleInput) => Promise<string | null>;
    onclose: () => void;
  }

  let { settings, members, readOnly, rules, rule, statuses, onsave, onclose }: Props = $props();

  const start = (): AutomationRuleInput => (rule ? ruleForm(rule) : newRuleForm(settings));
  let form = $state(start());
  let error = $state("");
  const disabled = $derived(readOnly);
  const title = $derived(readOnly ? readOnlyNote : undefined);

  async function save() {
    const refusal = ruleFormRefusal(form, rules, rule?.id ?? "");
    if (refusal) {
      error = refusal;
      return;
    }
    const message = await onsave({ ...form, name: form.name.trim() });
    if (message === null) onclose();
    else error = message;
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
        options={statuses}
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
      options={valueOptions(form, settings, members)}
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
