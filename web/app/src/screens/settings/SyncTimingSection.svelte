<script lang="ts">
  import {
    afterWriteChoices,
    githubChoices,
    intervalChoices,
    minutesLabel,
    type SyncTiming,
    secondsLabel,
    syncCard,
  } from "@hedgehug/core";
  import { Button, Checkbox, Select } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { syncTiming } from "../../settings";
  import { sync, syncNow } from "../../wire";

  // 「同期のタイミング」(08「一般」)。選んだらすぐに保存する。チェックがオフなら 3 つの選択を無効にする。
  const timing = syncTiming.state;
  onMount(() => {
    void syncTiming.load();
  });

  // 「すべて同期」は、remote を持つ表示中の Store があるときだけ出す。同期カードが出る条件と同じ(G1-20)。
  const status = sync.status;
  const canSync = $derived(syncCard($status, $syncNow) !== null);

  const options = (choices: readonly number[], label: (n: number) => string) =>
    choices.map((n) => ({ value: String(n), label: label(n) }));
  const selects: { key: keyof SyncTiming; label: string; options: ReturnType<typeof options> }[] = [
    { key: "intervalMinutes", label: "間隔", options: options(intervalChoices, minutesLabel) },
    {
      key: "afterWriteSeconds",
      label: "書き込みのあと同期するまで",
      options: options(afterWriteChoices, secondsLabel),
    },
    {
      key: "githubMinutes",
      label: "GitHub を読む間隔",
      options: options(githubChoices, minutesLabel),
    },
  ];
</script>

<section class="timing" aria-labelledby="sync-timing-title">
  <h2 id="sync-timing-title">同期のタイミング</h2>
  <p class="note">このマシンだけの設定です。</p>
  {#if $timing}
    {@const t = $timing}
    <Checkbox
      label="裏で自動的に同期する"
      checked={t.auto}
      onchange={(auto) => void syncTiming.set({ auto })}
    />
    {#each selects as s (s.key)}
      <div class="row">
        <span>{s.label}</span>
        <Select
          label={s.label}
          size="sm"
          options={s.options}
          disabled={!t.auto}
          value={String(t[s.key])}
          onchange={(v) => void syncTiming.set({ [s.key]: Number(v) })}
        />
      </div>
    {/each}
  {/if}
  {#if canSync}
    <div>
      <Button size="sm" onclick={() => void sync.syncAll()}>すべて同期</Button>
    </div>
  {/if}
</section>

<style>
  .timing {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  h2 {
    margin: 0;
    font-size: 15px;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: 12.5px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .row > span {
    min-width: 200px;
    font-size: 12.5px;
    color: var(--ink-2);
  }
</style>
