<script lang="ts">
  import {
    appMissingBandText,
    copiedText,
    copyFailedText,
    installRequestText,
    stillUnreadableText,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { toasts } from "../../wire";

  // 「GitHub App 未導入」の帯(07「GitHub App が入っていない組織」)。組織ごとに 1 本。
  // 「管理者にインストールを依頼」で依頼文をコピーする。コピーできなければ、帯の中に依頼文を出す。
  interface Props {
    name: string;
    login: string;
    org: string;
    still: boolean;
  }

  let { name, login, org, still }: Props = $props();
  const request = $derived(installRequestText(org, login));
  let copyFailed = $state(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(request);
      copyFailed = false;
      toasts.show(copiedText);
    } catch {
      copyFailed = true;
    }
  }
</script>

<div class="band" role="status">
  <span>{appMissingBandText(name, org)}</span>
  <Button size="xs" onclick={() => void copy()}>管理者にインストールを依頼</Button>
  {#if still}
    <p class="error" role="alert">{stillUnreadableText}</p>
  {/if}
  {#if copyFailed}
    <p>{copyFailedText}</p>
    <p class="request">{request}</p>
  {/if}
</div>

<style>
  .band {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    margin-bottom: 8px;
    padding: 8px 12px;
    border-radius: 8px;
    background: var(--c-orange-bg);
    color: var(--c-orange-fg);
    font-size: 12.5px;
  }
  p {
    flex-basis: 100%;
    margin: 0;
  }
  .error {
    color: var(--danger);
  }
  .request {
    user-select: text;
  }
</style>
