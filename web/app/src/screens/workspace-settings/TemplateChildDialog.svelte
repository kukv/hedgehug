<script lang="ts">
  import { type ChildForm, type SettingsState, valueFields } from "@hedgehug/core";
  import { Button, Dialog } from "@hedgehug/ui";
  import TemplateSteps from "./TemplateSteps.svelte";
  import TemplateValueFields from "./TemplateValueFields.svelte";

  // 子の説明・値・Step(03「一覧と編集」)。保存は、親のダイアログの「保存」で行う。
  interface Props {
    child: ChildForm;
    state: SettingsState;
    disabled: boolean;
    title?: string;
    onclose: () => void;
  }

  let { child = $bindable(), state: s, disabled, title, onclose }: Props = $props();

  const fields = $derived(valueFields(s.detail.settings, child.kind, s.members));
</script>

<Dialog open wide title={child.title.trim() || child.kind} {onclose}>
  <div class="body">
    <textarea
      aria-label="説明"
      rows="4"
      {disabled}
      {title}
      bind:value={child.description}
    ></textarea>
    <TemplateValueFields {fields} {disabled} {title} bind:values={child.values} />
    <TemplateSteps members={s.members} {disabled} {title} bind:steps={child.steps} />
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>閉じる</Button>
  {/snippet}
</Dialog>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 10px;
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
  .grow {
    flex: 1;
  }
</style>
