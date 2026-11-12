<script lang="ts">
  import type { SettingsTab } from "@hedgehug/core";
  import { PageHead, Tabs } from "@hedgehug/ui";
  import { navigation } from "../../wire";

  // 設定の画面(08「設定画面の構成」)。タブは、作ったものだけを出す(D2 画面の計画の決定 S1・S4)。
  interface Props {
    tab: SettingsTab;
    ondrawer: () => void;
  }

  let { tab, ondrawer }: Props = $props();
  const tabs = [{ id: "accounts", label: "アカウントとメンバー" }];
</script>

<PageHead title="設定" note="このマシンの設定と、Store の管理" {ondrawer} />
<Tabs
  {tabs}
  current={tab}
  onselect={(id) => navigation.go({ name: "settings", tab: id as SettingsTab })}
/>
<div class="content"></div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  @container frame (max-width: 860px) {
    .content {
      padding: 12px max(16px, env(safe-area-inset-right, 0px)) 32px
        max(16px, env(safe-area-inset-left, 0px));
    }
  }
</style>
