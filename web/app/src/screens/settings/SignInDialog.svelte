<script lang="ts">
  import {
    askingText,
    copiedMessage,
    copyFailedMessage,
    deniedText,
    expiredText,
    waitingText,
  } from "@hedgehug/core";
  import { Button, Dialog } from "@hedgehug/ui";
  import { signIn, toasts } from "../../wire";

  // 「GitHub にサインイン」(07「デバイスフロー」)。許可を待つ問い合わせは core の signIn が行う。
  const dialog = signIn.state;

  // コピーの結果は、詳細パネルの ID のコピーと同じ文言(D2 画面の計画の決定 S17)。
  async function copy(code: string) {
    try {
      await navigator.clipboard.writeText(code);
      toasts.show(copiedMessage(code));
    } catch {
      toasts.show(copyFailedMessage(code), { error: true });
    }
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="GitHub にサインイン" onclose={signIn.close}>
    <p class="line">このコードを GitHub で入力してください。</p>
    <div class="code-row">
      <code class="code">{s.code.userCode}</code>
      <Button size="sm" onclick={() => void copy(s.code.userCode)}>コピー</Button>
    </div>
    <p class="line">
      <a href={s.code.verificationUri} target="_blank" rel="noopener noreferrer"
        >{s.code.verificationUri}</a
      >
      を開き、コードを入力して許可します。
    </p>
    {#if s.retrying}
      <p class="muted">{askingText}</p>
    {:else if s.ended === "expired"}
      <p class="form-error" role="alert">{expiredText}</p>
    {:else if s.ended === "denied"}
      <p class="form-error" role="alert">{deniedText}</p>
    {:else if s.ended === "failed"}
      <p class="form-error" role="alert">{s.error}</p>
    {:else}
      <p class="muted">{waitingText}</p>
    {/if}
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={signIn.close}>キャンセル</Button>
      {#if s.ended}
        <Button variant="primary" disabled={s.retrying} onclick={() => void signIn.retry()}>
          再試行
        </Button>
      {/if}
    {/snippet}
  </Dialog>
{/if}

<style>
  .line {
    margin: 0;
  }
  .code-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .code {
    font-family: var(--font-mono);
    font-size: 22px;
    letter-spacing: 0.08em;
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .muted {
    color: var(--muted);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
