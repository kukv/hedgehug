<script lang="ts">
  import {
    addableTypes,
    keyHint,
    newProperty,
    type PropertyType,
    propertyKinds,
    typeLabels,
  } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input, Select } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";

  // 「プロパティを追加」(03「追加プロパティ」)。断りはダイアログの中に赤字で出す。
  interface Props {
    // 追加できたら、追加したプロパティの ID と型を渡す。
    onadded: (id: string, type: PropertyType) => void;
    onclose: () => void;
  }

  let { onadded, onclose }: Props = $props();

  let name = $state("");
  let key = $state("");
  let type = $state<PropertyType>("text");
  let chosen = $state<string[]>([]);
  let error = $state("");

  const typeOptions = addableTypes.map((t) => ({ value: t, label: typeLabels[t] }));
  const toggle = (kind: string, on: boolean) => {
    chosen = on ? [...chosen, kind] : chosen.filter((k) => k !== kind);
  };

  async function add() {
    error = "";
    let created: { id: string; type: PropertyType } | undefined;
    const ok = await workspaceSettings.run(
      async (p, s, w) => {
        const x = await p.addProperty(s, w, newProperty(name, key, type, chosen));
        created = { id: x.id, type: x.type };
      },
      "プロパティを追加しました",
      (message) => (error = message),
    );
    if (ok && created) onadded(created.id, created.type);
  }
</script>

<Dialog open title="プロパティを追加" {onclose}>
  <Input label="名前" bind:value={name} focusOnMount />
  <Input label="キー" bind:value={key} mono />
  <p class="hint">{keyHint}</p>
  <Select label="型" bind:value={type as string} options={typeOptions} />
  <fieldset>
    <legend>使う種類</legend>
    {#each propertyKinds as kind (kind)}
      <Checkbox label={kind} bind:checked={() => chosen.includes(kind), (on) => toggle(kind, on)} />
    {/each}
  </fieldset>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" onclick={add}>追加</Button>
  {/snippet}
</Dialog>

<style>
  .hint {
    margin: -8px 0 0;
    font-size: 11.5px;
    color: var(--muted);
  }
  fieldset {
    border: 0;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 6px 14px;
  }
  legend {
    font-size: 12.5px;
    color: var(--ink-2);
    padding: 0;
    margin-bottom: 6px;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
