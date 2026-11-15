<script lang="ts">
  import { clockWarning } from "@hedgehug/core";
  import { ConfirmDialog, Input } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { loginField } from "../../settings";
  import { sync } from "../../wire";
  import KeyTables from "./KeyTables.svelte";
  import SyncTimingSection from "./SyncTimingSection.svelte";

  // 設定の「一般」(08「一般」)。上から、ログイン名・時計のずれの警告・同期のタイミング・キーボードの一覧。
  const status = sync.status;
  const warning = $derived(clockWarning($status));
  const login = loginField.state;
  onMount(() => {
    void loginField.load();
  });

  // Enter(日本語変換の Enter は除く)とフォーカスが外れたときに確定する。続けて来た 2 回目は core が捨てる(G1-15)。
  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void loginField.commit($login.text);
  }
</script>

<div class="general">
  <div class="field">
    <label for="settings-login">ログイン名</label>
    <Input
      id="settings-login"
      label="ログイン名"
      {onkeydown}
      onblur={() => void loginField.commit($login.text)}
      bind:value={() => $login.text, (v) => loginField.edit(v)}
    />
    {#if $login.error}
      <p class="form-error" role="alert">{$login.error}</p>
    {/if}
  </div>
  {#if warning}
    <p class="warn">{warning}</p>
  {/if}
  <SyncTimingSection />
  <KeyTables />
</div>
{#if $login.confirm}
  <ConfirmDialog
    title="ログイン名を変更"
    lines={$login.confirm}
    ok="変更"
    danger
    onok={() => void loginField.confirm()}
    oncancel={loginField.cancel}
  />
{/if}

<style>
  .general {
    display: flex;
    flex-direction: column;
    gap: 20px;
    max-width: 860px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-width: 320px;
  }
  .field > label {
    font-weight: 600;
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .warn {
    margin: 0;
    color: var(--c-amber-fg);
  }
</style>
