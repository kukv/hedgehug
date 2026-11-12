<script lang="ts">
  import type { ItemDialogState, PropertyInput } from "@hedgehug/core";
  import { itemDialog } from "../../wire";
  import PropertyField from "./PropertyField.svelte";

  // 作成ダイアログのプロパティ欄(02「ダイアログの項目」、決定 25)。ステータスはセレクトで出す。
  interface Props {
    s: ItemDialogState;
  }

  let { s }: Props = $props();

  function set(id: string, value: PropertyInput) {
    void itemDialog.update({ values: { ...s.form.values, [id]: value } });
  }
  function toggle(id: string, valueId: string, on: boolean) {
    const now = (s.form.values[id] as string[] | undefined) ?? [];
    set(id, on ? [...now, valueId] : now.filter((x) => x !== valueId));
  }
</script>

<div class="create-props">
  {#each s.rows as row (row.property.id)}
    <div class="create-prop">
      <span>{row.property.name}</span>
      <PropertyField
        {row}
        settings={s.settings}
        disabled={false}
        statusSelect
        onchange={(value) => set(row.property.id, value)}
        ontoggle={(valueId, on) => toggle(row.property.id, valueId, on)}
      />
    </div>
  {/each}
</div>

<style>
  .create-props {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 12px;
  }
  .create-prop {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .create-prop > span {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
