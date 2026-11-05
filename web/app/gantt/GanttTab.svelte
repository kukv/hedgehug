<script lang="ts">
  import { DEFAULT_GANTT, type GanttSettings } from "../../core/views";
  import SaveViewButton from "../list/SaveViewButton.svelte";
  import type { ListScope } from "../list/scope";
  import GanttView from "./GanttView.svelte";

  // The gantt tab of a workspace or a project. The settings are kept for as long as the screen is
  // open and not saved, as the kanban's and the table's are (D7).
  type Props = {
    view: Extract<ListScope, { kind: "workspace" | "project" }>;
    query: string;
    onquery: (q: string) => void;
  };
  let { view, query, onquery }: Props = $props();

  let gantt = $state.raw<GanttSettings>(DEFAULT_GANTT);
</script>

<GanttView {view} {query} {onquery} settings={gantt} onsettings={(s) => (gantt = s)}>
  {#snippet actions()}
    <SaveViewButton {view} layout="gantt" filter={query} {gantt} />
  {/snippet}
</GanttView>
