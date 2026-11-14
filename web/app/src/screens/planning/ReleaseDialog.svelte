<script lang="ts">
  import { type Release, type ReleaseInput, releaseCreated } from "@hedgehug/core";
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { releaseAction } from "../../planning";
  import { planningPort } from "../../wire";

  // リリースの作成と編集のダイアログ(05「作成と編集」)。検査はサーバーが断り、その文をダイアログの中に出す。
  // 作成は「{名前} を作成しました」、編集はトーストを出さない(F1-18)。
  interface Props {
    storeId: string;
    workspaceId: string;
    release: Release | null; // null は作成
    // initial は、初めの値(作成は空、編集はリリースの値)。
    initial: ReleaseInput;
    onclose: () => void;
  }

  let { storeId, workspaceId, release, initial, onclose }: Props = $props();

  const start = () => ({ ...initial });
  let form = $state(start());
  let error = $state("");
  let busy = $state(false);

  async function save() {
    if (busy) return;
    busy = true;
    const input = { ...form };
    const ok = await releaseAction(
      async () => {
        if (release) {
          await planningPort.updateRelease(storeId, workspaceId, release.id, input);
          return null;
        }
        const made = await planningPort.createRelease(storeId, workspaceId, input);
        return releaseCreated(made.name);
      },
      (message) => (error = message),
    );
    busy = false;
    if (ok) onclose();
  }
</script>

<Dialog open title={release ? "リリースを編集" : "リリースを作成"} {onclose}>
  <div class="form">
    <div class="field">
      <span>名前</span>
      <Input label="名前" placeholder="v2.1.0" bind:value={form.name} focusOnMount />
    </div>
    <div class="field">
      <span>予定日</span>
      <Input label="予定日" type="date" bind:value={form.date} />
    </div>
    <div class="field">
      <span>説明</span>
      <Input label="説明" bind:value={form.description} />
    </div>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={save}>{release ? "保存" : "作成"}</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
