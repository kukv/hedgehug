<script lang="ts">
  import {
    get,
    prefixConfirm,
    prefixExamples,
    prefixHelp,
    readOnlyNote,
    routeOfWorkspace,
    type SettingsState,
  } from "@hedgehug/core";
  import { Button, Input } from "@hedgehug/ui";
  import { catalog, navigation, workspaceSettings } from "../../wire";
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

  const examples = $derived(prefixExamples(s.detail.prefix, s.usage.firstNumbers));
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
    if (workspaceSettings.askPrefix(prefix)) confirming = prefix;
  }
  function prefixKey(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) askPrefix();
  }
  // 検査の断りはエラーのトーストなので、確認ダイアログを閉じてから run する。
  // 変えたら、アドレスの接頭辞も新しいものにする(古いままだと、この画面が開けなくなる)。
  async function applyPrefix() {
    const value = confirming;
    confirming = null;
    if (value === null) return;
    const storeId = s.storeId;
    const workspaceId = s.detail.id;
    const ok = await workspaceSettings.run(
      (p, st, w) => p.changePrefix(st, w, value),
      "接頭辞を変えました",
    );
    if (ok)
      navigation.replace(routeOfWorkspace(get(catalog.stores), storeId, workspaceId, "settings"));
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
    <p class="hint">
      {#if examples.length === 0}
        アイテムはまだありません
      {:else}
        今の例:
        {#each examples as id (id)}
          <code>{id}</code>{" "}
        {/each}
      {/if}
    </p>
    <p class="hint">{prefixHelp}</p>
  </div>
</div>

{#if confirming !== null}
  <PrefixDialog
    text={prefixConfirm(s.detail.prefix, confirming, s.usage)}
    onconfirm={() => void applyPrefix()}
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
  code {
    font-family: var(--font-mono);
  }
  .hint {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
