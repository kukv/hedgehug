<script lang="ts">
  import { PageHead } from "@hedgehug/ui";
  import { views } from "../../views";

  // ビューの画面(仮の中身。内容は E3b Task 6)。見出しにビューの名前を出す。
  interface Props {
    id: string;
    ondrawer: () => void;
    alert: boolean;
  }

  let { id, ondrawer, alert }: Props = $props();

  const list = views.list;
  const view = $derived($list.find((v) => v.id === id));
</script>

<PageHead title={view?.name ?? ""} {ondrawer} {alert} />
