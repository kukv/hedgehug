<script lang="ts">
  import {
    prefixConfirm,
    prefixExamples,
    prefixHelp,
    readOnlyNote,
    type SettingsState,
  } from "@hedgehug/core";
  import { Button, Input } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import PrefixDialog from "./PrefixDialog.svelte";

  // 「一般」の名前と接頭辞(03「名前」「接頭辞」)。入力の途中の値だけを持ち、操作は core の run に渡す。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  let name = $state("");
  let prefix = $state("");
  let confirming = $state<string | null>(null);

  // 読み直しのたびに、入力を保存済みの値に戻す(断りのときの巻き戻しもこれ)。
  $effect(() => {
    name = s.detail.name;
    prefix = s.detail.prefix;
  });

  const title = $derived(s.readOnly ? readOnlyNote : undefined);

  function commitName() {
    if (name === s.detail.name) return;
    const value = name;
    void workspaceSettings.run((p, st, w) => p.rename(st, w, value));
  }
  function nameKey(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.target as HTMLElement).blur();
  }
  function askPrefix() {
    if (prefix === s.detail.prefix) return;
    confirming = prefix;
  }
  function prefixKey(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) askPrefix();
  }
  // 検査の断りはエラーのトーストなので、確認ダイアログを閉じてから run する。
  function applyPrefix() {
    const value = confirming;
    confirming = null;
    if (value === null) return;
    void workspaceSettings.run((p, st, w) => p.changePrefix(st, w, value), "接頭辞を変えました");
  }
</script>

<div class="panel">
  <div class="field">
    <label for="ws-name">名前</label>
    <Input
      id="ws-name"
      label="名前"
      bind:value={name}
      disabled={s.readOnly}
      {title}
      onblur={commitName}
      onkeydown={nameKey}
    />
  </div>
  <div class="field">
    <label for="ws-prefix">接頭辞</label>
    <div class="row">
      <Input
        id="ws-prefix"
        label="接頭辞"
        mono
        bind:value={() => prefix, (v) => (prefix = v.toUpperCase())}
        disabled={s.readOnly}
        {title}
        onkeydown={prefixKey}
      />
      <Button disabled={s.readOnly} {title} onclick={askPrefix}>変更</Button>
    </div>
    <p class="hint">例: {prefixExamples(s.detail.prefix, s.usage.firstNumbers)}</p>
    <p class="hint">{prefixHelp}</p>
  </div>
</div>

{#if confirming !== null}
  <PrefixDialog
    text={prefixConfirm(s.detail.prefix, confirming, s.usage)}
    onconfirm={applyPrefix}
    onclose={() => (confirming = null)}
  />
{/if}

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 20px;
    max-width: 520px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > label {
    font-size: 12.5px;
    color: var(--muted);
  }
  .row {
    display: flex;
    gap: 8px;
  }
  .hint {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
