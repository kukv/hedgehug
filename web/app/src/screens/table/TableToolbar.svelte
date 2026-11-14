<script lang="ts">
  import {
    columnChoices,
    groupChoices,
    type SearchColumn,
    type TableSettings,
    toggleColumn,
  } from "@hedgehug/core";
  import { Button, Select, ValueMenu } from "@hedgehug/ui";
  import type { Snippet } from "svelte";
  import { pressWhileOpen } from "../item/pressWhileOpen";

  // テーブルのツールバー(04「列」「グループ」、E2a-14・E2a-15)。左から「表示する列」・「グループ」・件数。
  interface Props {
    count: number | null;
    // columns は対象の列すべて。
    columns: SearchColumn[];
    table: TableSettings;
    onchange: (next: TableSettings) => void;
    // children は、画面ごとの操作。
    children?: Snippet;
  }

  let { count, columns, table, onchange, children }: Props = $props();
  let menu = $state<{ x: number; y: number } | null>(null);
  // メニューは外を押すと閉じる(ボタンも含む)。開いているときにボタンを押したら、開き直さず閉じたままにする。
  let wasOpen = false;
  const groups = groupChoices.map((g) => ({ value: g.id, label: g.label }));

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

<div class="toolbar">
  <Button
    size="sm"
    onpointerdown={() => pressWhileOpen(menu !== null, (v) => (wasOpen = v))}
    onclick={open}
    >表示する列</Button
  >
  <Select
    size="sm"
    label="グループ"
    value={table.group}
    options={groups}
    onchange={(group) => onchange({ ...table, group })}
  />
  {#if count !== null}
    <span class="count">{count} 件</span>
  {/if}
  {@render children?.()}
</div>
<ValueMenu
  open={menu !== null}
  x={menu?.x ?? 0}
  y={menu?.y ?? 0}
  label="表示する列"
  multi
  options={columnChoices(table, columns)}
  onselect={(key) => onchange(toggleColumn(table, key))}
  onclose={() => (menu = null)}
/>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 24px;
  }
  .count {
    color: var(--muted);
    font-size: 12.5px;
    font-variant-numeric: tabular-nums;
  }
  @container frame (max-width: 860px) {
    .toolbar {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
