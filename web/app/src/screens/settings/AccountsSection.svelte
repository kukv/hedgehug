<script lang="ts">
  import { Button, ConfirmDialog } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { accountForm, accountList } from "../../accounts";
  import AccountDialog from "./AccountDialog.svelte";
  import AccountItem from "./AccountItem.svelte";
  import AppMissingBand from "./AppMissingBand.svelte";
  import SignInDialog from "./SignInDialog.svelte";

  // 「GitHub アカウント」の区画(07「アカウント」)。開くたびに行の赤字を消して読み直す(決定 S14)。
  const list = accountList.state;
  onMount(() => {
    void accountList.open();
  });
</script>

<section class="card" aria-labelledby="accounts-title">
  <div class="head">
    <h2 id="accounts-title">GitHub アカウント</h2>
    <span class="grow"></span>
    <Button size="sm" onclick={accountForm.openAdd}>追加</Button>
  </div>
  {#each $list.rows.filter((r) => r.status === "app_missing") as row (row.id)}
    {#each row.orgs as org (org)}
      <AppMissingBand
        name={row.name}
        login={row.login}
        {org}
        still={$list.still[row.id]?.includes(org) ?? false}
      />
    {/each}
  {/each}
  <ul class="rows" aria-label="GitHub アカウント">
    {#each $list.rows as row (row.id)}
      <AccountItem
        {row}
        error={$list.errors[row.id] ?? ""}
        asking={$list.asking.includes(row.id)}
        onreauth={() => void accountList.reauth(row.id)}
        onrecheck={() => void accountList.recheck(row)}
        onedit={() => {
          accountList.touch(row.id);
          accountForm.openEdit(row);
        }}
        ondelete={() => void accountList.askDelete(row.id)}
      />
    {/each}
  </ul>
</section>
<AccountDialog />
<SignInDialog />
{#if $list.deleting}
  <ConfirmDialog
    title="アカウントを削除"
    lines={$list.deleting.lines}
    ok="削除"
    danger
    onok={() => void accountList.confirmDelete()}
    oncancel={accountList.cancelDelete}
  />
{/if}

<style>
  .card {
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: 12px;
    padding: 14px 16px 6px;
    max-width: 860px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .head h2 {
    margin: 0;
    font-size: 15px;
  }
  .grow {
    flex: 1;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
</style>
