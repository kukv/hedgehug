<script lang="ts">
  import {
    type ItemPropertyRow,
    type PropertyInput,
    readOnlyNote,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Input, Select, ValueMenu } from "@hedgehug/ui";
  import { pressWhileOpen } from "./pressWhileOpen";
  import StatusButton from "./StatusButton.svelte";

  // 1 つのプロパティの入力(02「入力の方法と保存」の表、決定 20)。onchange は、選ぶもの・チェックは選んだとき、
  // 入力欄はフォーカスが外れたとき(Enter でも)。ontoggle は関連システム・複数選択の入り切り。
  // statusSelect なら、ステータスをセレクトで出す(作成ダイアログ。02「ダイアログの項目」)。
  // itemId は、詳細パネルで値を持つアイテム。project は、そのプロジェクト(ステータスのメニューを絞る)。入力欄は、フォーカスしたときの itemId を onchange に添える。
  interface Props {
    row: ItemPropertyRow;
    itemId?: string;
    project: string;
    settings: WorkspaceSettings;
    disabled: boolean;
    statusSelect?: boolean;
    onchange: (value: PropertyInput, itemId?: string) => void;
    ontoggle: (valueId: string, on: boolean) => void;
  }

  let {
    row,
    itemId,
    project,
    settings,
    disabled,
    statusSelect = false,
    onchange,
    ontoggle,
  }: Props = $props();
  const p = $derived(row.property);
  const note = $derived(disabled ? readOnlyNote : undefined);
  const multi = $derived(p.type === "systems" || p.type === "multi");
  const builtinNumber = $derived(p.builtin && (p.key === "points" || p.key === "estimate"));
  const shown = $derived(typeof row.value === "string" ? row.value : "");
  let draft = $state("");
  let selected = $state("");
  let menu = $state<{ x: number; y: number } | null>(null);
  // メニューは、外を押すと閉じる(ボタンを押したときも)。開いているときにボタンを押したら、開き直さず閉じたままにする。
  let wasOpen = false;
  const checkedId = $derived(row.options.find((o) => o.checked)?.id ?? shown);
  // editing は、入力欄にフォーカスがあるあいだ。画面への反映では、編集中の入力欄を書き換えない(06)。
  // $state にしない(フォーカスが外れたときに $effect が走り直し、保存の前に戻すため)。base は、フォーカスしたときの値。
  let editing = false;
  let base = "";
  let focusedItem: string | undefined;
  $effect(() => {
    const next = shown;
    const picked = checkedId;
    if (!editing) draft = next;
    selected = picked;
  });
  function focused(event: FocusEvent) {
    editing = true;
    base = (event.currentTarget as HTMLInputElement).value;
    focusedItem = itemId;
  }

  function openMenu(event: MouseEvent) {
    if (wasOpen) {
      wasOpen = false;
      menu = null;
      return;
    }
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    menu = { x: r.left, y: r.bottom + 4 };
  }
  function pick(id: string) {
    if (multi) ontoggle(id, !row.options.find((o) => o.id === id)?.checked);
    else if (id !== row.value) onchange(id);
  }
  // 入力欄の値は、入力欄そのものから文字列で読む(数も文字列で送る。決定 12)。
  // 数の欄に数でない文字があるとき: ポイント・見積もり時間はサーバーに断らせ(決定 16)、ほかは送らずに戻す。
  function commit(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    editing = false;
    // 打ち換えずに離れたら保存せず、反映で受け取った最新の値に合わせる。
    if (input.value === base) {
      draft = shown;
      input.value = shown;
      return;
    }
    const bad = input.validity?.badInput ?? false;
    if (bad && !builtinNumber) {
      draft = shown;
      input.value = shown;
      return;
    }
    const value = bad ? "x" : input.value;
    if (value !== shown) onchange(value, focusedItem);
  }
  function enter(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }
</script>

{#if p.type === "status" && !statusSelect}
  <StatusButton status={String(row.value)} {settings} {project} {disabled} {onchange} />
{:else if p.type === "status" || p.type === "user"}
  <Select
    label={p.name}
    {disabled}
    title={note}
    options={row.options.map((o) => ({ value: o.id, label: o.label }))}
    bind:value={selected}
    onchange={(value) => {
      if (value !== row.options.find((o) => o.checked)?.id) onchange(value);
    }}
  />
{:else if row.options.length > 0 || multi || p.type === "priority" || p.type === "single"}
  <button
    type="button"
    class="value-btn"
    class:deleted={row.deleted}
    {disabled}
    title={note}
    aria-label="{p.name}: {row.text}"
    onpointerdown={() => pressWhileOpen(menu !== null, (v) => (wasOpen = v))}
    onclick={openMenu}
  >
    {row.text}
  </button>
  <ValueMenu
    open={menu !== null}
    x={menu?.x ?? 0}
    y={menu?.y ?? 0}
    label={p.name}
    options={row.options}
    {multi}
    onselect={pick}
    onclose={() => (menu = null)}
  />
{:else if p.type === "check"}
  <input
    type="checkbox"
    aria-label={p.name}
    checked={row.value === true}
    {disabled}
    title={note}
    onchange={() => onchange(row.value !== true)}
  >
{:else if p.type === "date" || p.type === "number" || p.type === "duration"}
  <input
    class="field"
    type={p.type === "date" ? "date" : "number"}
    step={p.type === "duration" ? "0.25" : "any"}
    aria-label={p.name}
    {disabled}
    title={note}
    value={draft}
    onfocus={focused}
    onblur={commit}
    onkeydown={enter}
  >
{:else}
  <Input
    label={p.name}
    {disabled}
    title={note}
    bind:value={draft}
    onfocus={focused}
    onblur={commit}
    onkeydown={enter}
  />
{/if}

<style>
  .value-btn {
    font-size: 12px;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .deleted {
    color: var(--muted);
  }
  .field {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .field:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
