<script lang="ts">
  import { fetchLine, fetchTimes, rateLine, slowedText } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useGitHubRepos, useIssueFields, useRuleStats, useRules } from "../github.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import IssueImportSection from "./IssueImportSection.svelte";
  import IssueSyncTable from "./IssueSyncTable.svelte";
  import RuleDialog from "./RuleDialog.svelte";
  import RuleList from "./RuleList.svelte";

  // The rules, the issue sync and the reads from GitHub of a Store.
  let { store }: { store: string } = $props();

  const c = core();
  const tone = useSubscribe(c.stores, () => c.stores.tone(store));
  const off = useReadOnlySentence(c.stores, () => store);
  const rules = useRules(() => store);
  const stats = useRuleStats(() => store);
  const fields = useIssueFields(() => store);
  const repos = useGitHubRepos(() => store);
  const schema = useSubscribe(c.schema, () => c.schema.get(store));

  let schemaError = $state.raw<unknown>(null);
  $effect(() => {
    c.schema.load(store).catch((e: unknown) => {
      schemaError = e;
    });
  });

  const times = $derived(fetchTimes(repos.current, new Set(repos.current.map((r) => r.repo))));
  const line = $derived(fetchLine(times, new Date()));
  const rate = $derived(rateLine(repos.current));
  const slowed = $derived(slowedText(times.slowedUntil, new Date()));

  let adding = $state(false);
</script>

<div class="page">
  <header class="head">
    <div class="title">
      <div class="name">
        <StoreBadge {store} tone={tone.current} />
        <h1>自動化と GitHub 連携</h1>
      </div>
      <p class="lead">
        <code>automations.yaml</code>
        に保存します。自動化による変更は「hedgehug-automation（実行したメンバー）」としてコミットされます。
      </p>
    </div>
    <span class="reads">
      <span>{line}</span>
      {#if rate}
        <span class="mono">{rate}</span>
      {/if}
    </span>
  </header>
  {#if slowed}
    <p class="slowed" role="status">{slowed}</p>
  {/if}

  {#if schemaError}
    <FormError message="スキーマを読めません · {errorMessage(schemaError)}" />
  {/if}

  <section class="block" aria-labelledby="rules-h">
    <div class="line">
      <h2 id="rules-h">ルール</h2>
      <span class="msg">
        上から順に評価 · プロジェクトごとの停止・追加はプロジェクトの設定から ·
        制約に反すると拒否して inbox に出します
      </span>
      <span class="end">
        <Button
          size="sm"
          disabled={off.current !== null || !schema.current}
          why={off.current ?? undefined}
          onclick={() => (adding = true)}
        >
          ルールを追加
        </Button>
      </span>
    </div>
    {#if rules.current && schema.current}
      <RuleList
        {store}
        rules={rules.current}
        stats={stats.current}
        schema={schema.current}
        off={off.current}
      />
    {:else}
      <p class="msg">読み込み中…</p>
    {/if}
  </section>

  <section class="block" aria-labelledby="sync-h">
    <h2 id="sync-h">Issue の同期</h2>
    {#if fields.current}
      <IssueSyncTable {store} fields={fields.current} off={off.current} />
    {:else}
      <p class="msg">読み込み中…</p>
    {/if}
  </section>

  <section class="block box" aria-labelledby="imp-h">
    <h2 id="imp-h">既存の Issue をまとめて取り込む</h2>
    <IssueImportSection {store} off={off.current} />
  </section>
</div>

{#if adding && schema.current}
  <RuleDialog {store} schema={schema.current} onclose={() => (adding = false)} />
{/if}

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-20);
    max-width: 60rem;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-10);
  }
  .title {
    display: flex;
    flex: 1 1 20rem;
    flex-direction: column;
    gap: var(--space-6);
  }
  .name {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .lead {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  code,
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .reads {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .slowed {
    margin: 0;
    padding: var(--space-8) var(--space-12);
    border-radius: var(--radius-8);
    background: var(--warning-weak);
    color: var(--warning);
    font-size: var(--fs-12);
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-10);
  }
  .box {
    padding: var(--space-16);
    border: 1px solid var(--line);
    border-radius: var(--radius-12);
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .end {
    margin-left: auto;
  }
</style>
