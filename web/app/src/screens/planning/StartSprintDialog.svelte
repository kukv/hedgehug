<script lang="ts">
  import { type SprintCard, sprintStarted, startBody, startDefaults } from "@hedgehug/core";
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { sprintAction } from "../../planning";
  import { planningPort } from "../../wire";

  // スプリントの開始のダイアログ(05「開始」)。日付の検査はサーバーが断り、その文をダイアログの中に出す(F1-8)。
  interface Props {
    storeId: string;
    workspaceId: string;
    card: SprintCard;
    // today は、このマシンの時刻帯での今日(YYYY-MM-DD)。
    today: string;
    onclose: () => void;
  }

  let { storeId, workspaceId, card, today, onclose }: Props = $props();

  const s = $derived(card.sprint);
  const initial = () => startDefaults(card.sprint, today);
  let dates = $state(initial());
  let error = $state("");
  let busy = $state(false);

  async function save() {
    if (busy) return;
    busy = true;
    const { id, name } = s;
    const { start, end } = dates;
    const ok = await sprintAction(
      async () => {
        await planningPort.startSprint(storeId, workspaceId, id, start, end);
        return sprintStarted(name);
      },
      (message) => (error = message),
    );
    busy = false;
    if (ok) onclose();
  }
</script>

<Dialog open title={s.name} {onclose}>
  <div class="form">
    <p class="text">{startBody(card)}</p>
    <div class="dates">
      <div class="field">
        <span>開始日</span>
        <Input label="開始日" type="date" bind:value={dates.start} />
      </div>
      <div class="field">
        <span>終了日</span>
        <Input label="終了日" type="date" bind:value={dates.end} />
      </div>
    </div>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={save}>開始</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .text {
    margin: 0;
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
