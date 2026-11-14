<script lang="ts">
  import { type Sprint, type SprintInput, sprintCreated } from "@hedgehug/core";
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { sprintAction } from "../../planning";
  import { planningPort } from "../../wire";

  // スプリントの作成と編集のダイアログ(05「作成と編集」)。検査はサーバーが断り、その文をダイアログの中に出す(F1-8)。
  // 作成は「{名前} を作成しました」、編集はトーストを出さない(F1-18)。
  interface Props {
    storeId: string;
    workspaceId: string;
    sprint: Sprint | null; // null は作成
    // initial は、初めの値(作成は PlanningOverview.defaults、編集はスプリントの値)。
    initial: SprintInput;
    onclose: () => void;
  }

  let { storeId, workspaceId, sprint, initial, onclose }: Props = $props();

  const start = () => ({ ...initial });
  let form = $state(start());
  let error = $state("");
  let busy = $state(false);

  async function save() {
    if (busy) return;
    busy = true;
    const input = { ...form };
    const ok = await sprintAction(
      async () => {
        if (sprint) {
          await planningPort.updateSprint(storeId, workspaceId, sprint.id, input);
          return null;
        }
        const made = await planningPort.createSprint(storeId, workspaceId, input);
        return sprintCreated(made.name);
      },
      (message) => (error = message),
    );
    busy = false;
    if (ok) onclose();
  }
</script>

<Dialog open title={sprint ? "スプリントを編集" : "スプリントを作成"} {onclose}>
  <div class="form">
    <div class="field">
      <span>名前</span>
      <Input label="名前" bind:value={form.name} focusOnMount />
    </div>
    <div class="dates">
      <div class="field">
        <span>開始日</span>
        <Input label="開始日" type="date" bind:value={form.start} />
      </div>
      <div class="field">
        <span>終了日</span>
        <Input label="終了日" type="date" bind:value={form.end} />
      </div>
    </div>
    <div class="field">
      <span>ゴール</span>
      <Input label="ゴール" bind:value={form.goal} />
    </div>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={save}>{sprint ? "保存" : "作成"}</Button>
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
  .dates {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
