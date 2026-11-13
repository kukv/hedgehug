<script lang="ts">
  import { type ProjectView, type RuleSection, readOnlyNote } from "@hedgehug/core";
  import { Button, ConfirmDialog } from "@hedgehug/ui";
  import { projectsPort, rulesEditor } from "../../projectEditor";

  // 区画の「ワークスペースの設定に戻す…」(03「ワークスペースの設定に戻す」)。確認ダイアログを出し、「戻す」で
  // 区画のルールをすべて削除する。区画にルールがない(empty)ときと閲覧のみの Store では無効。
  interface Props {
    view: ProjectView;
    section: RuleSection;
    // 本文の {区画名}(例「使うステータス」)
    label: string;
    empty: boolean;
    // 本文に続ける文(自動化の「このプロジェクトだけのルール {N} 件も削除します。」。N が 0 なら渡さない)
    extra?: string;
  }

  let { view, section, label, empty, extra }: Props = $props();

  let open = $state(false);
  const readOnly = $derived(view.store.readOnly);

  const lines = $derived([
    `このプロジェクトの${label}のルールをすべて削除し、${view.workspace.name} の設定に戻します。`,
    ...(extra ? [extra] : []),
  ]);

  async function reset() {
    open = false;
    const p = view.project;
    await rulesEditor.run(
      () => projectsPort.resetRules(p.storeId, p.id, section),
      "ワークスペースの設定に戻しました",
    );
  }
</script>

<div>
  <Button
    disabled={readOnly || empty}
    title={readOnly ? readOnlyNote : undefined}
    onclick={() => (open = true)}
    >ワークスペースの設定に戻す…</Button
  >
</div>

{#if open}
  <ConfirmDialog
    title="ワークスペースの設定に戻す"
    {lines}
    ok="戻す"
    onok={() => void reset()}
    oncancel={() => (open = false)}
  />
{/if}
