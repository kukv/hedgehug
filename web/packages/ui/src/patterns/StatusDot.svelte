<script lang="ts">
  import type { Category } from "../types";

  interface Props {
    category: Category;
    // label は、読み上げに使う名前(例 "InProgress")。
    label: string;
  }

  let { category, label }: Props = $props();
</script>

<i
  class="sdot"
  class:todo={category === "todo"}
  class:doing={category === "doing"}
  class:done={category === "done"}
  class:canceled={category === "canceled"}
  role="img"
  aria-label={label}
  title={label}
></i>

<style>
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    flex: none;
  }
  .todo {
    border: 1.5px solid var(--cat-todo);
  }
  .doing {
    background: var(--cat-doing);
    box-shadow: inset 0 0 0 2px var(--surface);
    border: 1.5px solid var(--cat-doing);
  }
  .done {
    background: var(--cat-done);
  }
  .canceled {
    background: var(--cat-canceled);
  }
</style>
