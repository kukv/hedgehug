<script lang="ts">
  import { installUrl, type MissingApp } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";

  // The GitHub App is not installed where an account reads. It goes on with the token of gh, or an
  // administrator installs the App; an account on gh already has only the second.
  type Props = { name: string; missing: MissingApp; canSwitch: boolean };
  let { name, missing, canSwitch }: Props = $props();

  const c = core();
  let busy = $state(false);

  async function useGh() {
    busy = true;
    try {
      await c.api.setAccountTokenSource(name, { token: "gh" });
      toast({ message: `${name} は次の取得から gh のトークンを使います` });
      await c.accounts.reload();
    } catch (e) {
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }
</script>

<div class="warn" role="note">
  <Icon name="warning" size={15} />
  <span class="text">
    {missing.repos.join("・")}
    に Hedgehug の GitHub App が入っていません。PR と Issue の状態を取り込めません。
  </span>
  {#if canSwitch}
    <Button size="sm" disabled={busy} onclick={() => void useGh()}>
      gh のトークンで代わりに使う
    </Button>
  {/if}
  <a href={installUrl(missing)} target="_blank" rel="noreferrer">管理者にインストールを依頼</a>
</div>

<style>
  .warn {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-8);
    background: var(--warning-weak);
    color: var(--warning);
    font-size: var(--fs-12);
  }
  .text {
    flex: 1 1 18rem;
    color: var(--ink);
    line-height: 1.6;
  }
</style>
