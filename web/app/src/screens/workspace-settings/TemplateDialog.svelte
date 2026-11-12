<script lang="ts">
  import {
    emptyTemplateForm,
    formOfTemplate,
    kindChangeNote,
    propertyKinds,
    readOnlyNote,
    type SettingsState,
    structureOf,
    templateCreated,
    toTemplateInput,
    valueFields,
    type WorkspaceTemplate,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, Dialog, Input, Select } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import TemplateChildren from "./TemplateChildren.svelte";
  import TemplateSteps from "./TemplateSteps.svelte";
  import TemplateValueFields from "./TemplateValueFields.svelte";

  // テンプレートの作成・編集(03「一覧と編集」)。断りはダイアログの中に赤字で出す。
  interface Props {
    state: SettingsState;
    // null は新規。開いたときの中身で欄を埋める。
    template: WorkspaceTemplate | null;
    onclose: () => void;
  }

  let { state: s, template, onclose }: Props = $props();

  const start = () => (template ? formOfTemplate(template) : emptyTemplateForm());
  let form = $state(start());
  let error = $state("");
  let confirming = $state<string | null>(null);
  let childOpen = $state<number | null>(null);

  const kindOptions = propertyKinds.map((k) => ({ value: k, label: k }));
  const fields = $derived(valueFields(s.detail.settings, form.kind, s.members));
  const structure = $derived(structureOf(form.kind));
  const disabled = $derived(s.readOnly);
  const title = $derived(s.readOnly ? readOnlyNote : undefined);

  // 子のダイアログや確認を開いている間の Escape では閉じない。
  function close() {
    if (childOpen === null && confirming === null) onclose();
  }

  function save() {
    if (s.readOnly) return;
    // 種類を変えたときに捨てる構成は、いまのフォームの構成で数える(B3-14)。
    const note = kindChangeNote($state.snapshot(form), form.kind);
    if (note === null) void commit();
    else confirming = note;
  }

  async function commit() {
    confirming = null;
    error = "";
    const input = toTemplateInput($state.snapshot(form), s.detail.settings);
    const id = form.id;
    const refused = (m: string) => (error = m);
    const ok =
      id === ""
        ? await workspaceSettings.run(
            (p, st, w) => p.createTemplate(st, w, input),
            templateCreated,
            refused,
          )
        : await workspaceSettings.run(
            (p, st, w) => p.updateTemplate(st, w, id, input),
            undefined,
            refused,
          );
    if (ok) onclose();
  }
</script>

<Dialog
  open
  wide
  title={form.id === "" ? "テンプレートを作成" : "テンプレートの編集"}
  onclose={close}
>
  <div class="body">
    <div class="field">
      <span class="k">名前</span>
      <Input label="名前" {disabled} {title} bind:value={form.name} focusOnMount={!disabled} />
    </div>
    <div class="field">
      <span class="k">種類</span>
      <Select label="種類" options={kindOptions} {disabled} {title} bind:value={form.kind} />
    </div>
    <div class="field">
      <span class="k">タイトルの書き出し</span>
      <Input label="タイトルの書き出し" {disabled} {title} bind:value={form.title} />
    </div>
    <div class="field">
      <span class="k">説明</span>
      <textarea
        aria-label="説明"
        rows="4"
        {disabled}
        {title}
        bind:value={form.description}
      ></textarea>
    </div>
    <TemplateValueFields {fields} {disabled} {title} bind:values={form.values} />
    {#if structure === "children"}
      <TemplateChildren
        state={s}
        {disabled}
        {title}
        bind:children={form.children}
        bind:open={childOpen}
      />
    {:else if structure === "steps"}
      <TemplateSteps members={s.members} {disabled} {title} bind:steps={form.steps} />
    {/if}
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

{#if confirming !== null}
  <ConfirmDialog
    title="テンプレートを保存"
    lines={[confirming]}
    ok="保存"
    onok={commit}
    oncancel={() => (confirming = null)}
  />
{/if}

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .field {
    display: grid;
    grid-template-columns: 120px 1fr;
    align-items: center;
    gap: 8px;
  }
  .k {
    font-size: 12.5px;
    color: var(--ink-2);
  }
  textarea {
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 6px 10px;
    resize: vertical;
  }
  textarea:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
