<script lang="ts">
  import { type ProjectView, readOnlyNote } from "@hedgehug/core";
  import { Button, IconButton, Input } from "@hedgehug/ui";
  import { projectEditor } from "../../projectEditor";

  // 「リポジトリと PR」の区画の上段(02「概要の編集」「リポジトリの追加と削除」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const disabled = $derived(view.store.readOnly);
  const hint = $derived(disabled ? readOnlyNote : undefined);

  // 追加の入力欄は、「リポジトリを追加」で開く。断られたら開いたまま、断りの文言を欄の下に出す。
  let adding = $state(false);
  let input = $state("");
  let error = $state("");

  function open() {
    adding = true;
    error = "";
  }

  function cancel() {
    adding = false;
    input = "";
    error = "";
  }

  async function add() {
    error = await projectEditor.addRepo(p, input);
    if (error === "") cancel();
  }

  // Enter で追加する。日本語の変換を確定する Enter は除く。
  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void add();
  }
</script>

<section class="card-sec" aria-label="リポジトリと PR">
  <div class="sec-head">
    <h2>リポジトリと PR</h2>
    <span class="grow"></span>
    <Button size="sm" {disabled} title={hint} onclick={open}>リポジトリを追加</Button>
  </div>
  {#if adding}
    <div class="add">
      <Input
        label="リポジトリ"
        size="sm"
        placeholder="owner/name"
        focusOnMount
        bind:value={input}
        {onkeydown}
      />
      <Button size="sm" variant="primary" onclick={() => void add()}>追加</Button>
      <Button size="sm" onclick={cancel}>キャンセル</Button>
    </div>
    {#if error !== ""}
      <p class="error" role="alert">{error}</p>
    {/if}
  {/if}
  {#if p.repos.length === 0}
    <p class="empty">リポジトリはまだありません</p>
  {:else}
    <ul class="repos" aria-label="リポジトリ">
      {#each p.repos as repo (repo)}
        <li class="repo">
          <code>{repo}</code>
          <span class="grow"></span>
          <IconButton
            label="{repo} を削除"
            size="xs"
            {disabled}
            title={hint}
            onclick={() => void projectEditor.removeRepo(p, repo)}
            >×</IconButton
          >
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .add {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .error {
    color: var(--danger);
    margin: 0 0 8px;
    font-size: 12.5px;
  }
  .empty {
    color: var(--muted);
    margin: 0;
  }
  .repos {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .repo {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  code {
    font-family: var(--font-mono);
    font-size: 12.5px;
  }
</style>
