<script lang="ts">
  import {
    deletedValue,
    readOnlyNote,
    statusOptions,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { StatusDot, ValueMenu } from "@hedgehug/ui";
  import { pressWhileOpen } from "./pressWhileOpen";

  // ステータスのボタンとメニュー(02「ステータスの変更」、決定 21)。いまの値を選び直しても保存しない。
  interface Props {
    status: string;
    settings: WorkspaceSettings;
    disabled: boolean;
    onchange: (statusId: string) => void;
  }

  let { status, settings, disabled, onchange }: Props = $props();
  let menu = $state<{ x: number; y: number } | null>(null);
  // メニューは、外を押すと閉じる(ボタンを押したときも)。開いているときにボタンを押したら、開き直さず閉じたままにする。
  let wasOpen = false;
  const current = $derived(settings.statuses.find((s) => s.id === status));
  const name = $derived(current?.name ?? deletedValue);

  function open(event: MouseEvent) {
    if (wasOpen) {
      wasOpen = false;
      menu = null;
      return;
    }
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    menu = { x: r.left, y: r.bottom + 4 };
  }
</script>

<button
  type="button"
  class="status-btn"
  class:deleted={!current}
  {disabled}
  title={disabled ? readOnlyNote : undefined}
  aria-label="ステータス: {name}"
  onpointerdown={() => pressWhileOpen(menu !== null, (v) => (wasOpen = v))}
  onclick={open}
>
  {#if current}
    <StatusDot category={current.category ?? "todo"} label={current.name} />
  {/if}
  {name}
</button>
<ValueMenu
  open={menu !== null}
  x={menu?.x ?? 0}
  y={menu?.y ?? 0}
  label="ステータス"
  options={statusOptions(settings, status)}
  onselect={(id) => {
    if (id !== status) onchange(id);
  }}
  onclose={() => (menu = null)}
/>

<style>
  .status-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
  }
  .deleted {
    color: var(--muted);
  }
</style>
