import type { Schemas } from "../../core/api/client";
import { cliGhImport, Placeholder } from "../../core/cli";
import { errorMessage } from "../../core/messages";
import { core } from "../context";
import { toast } from "../toast.svelte";

/** Where the tasks go: under a task (parentId is its ID as the command writes it). */
export type ImportPlace = { parent?: string; parentId?: string };

/**
 * The form of hh gh import. The count comes first (dry_run): what the button says is what it takes
 * in. done runs after the tasks were made.
 */
export function useIssueImport(
  store: () => string,
  place: () => ImportPlace,
  done: () => void,
): {
  repo: string;
  label: string;
  readonly counted: Schemas.ImportResult | null;
  readonly busy: boolean;
  readonly error: string;
  readonly ready: boolean;
  readonly cli: string;
  forget: () => void;
  run: (dry: boolean) => Promise<void>;
} {
  const c = core();
  let repo = $state("");
  let label = $state("");
  let counted = $state<Schemas.ImportResult | null>(null);
  let busy = $state(false);
  let error = $state("");

  const ready = $derived(repo.trim() !== "" && label.trim() !== "");
  const cli = $derived(
    cliGhImport({
      repo: repo.trim() || new Placeholder("org/repo"),
      label: label.trim() || new Placeholder("ラベル"),
      parent: place().parentId,
    }),
  );

  async function run(dry: boolean) {
    if (!ready || busy) return;
    busy = true;
    error = "";
    try {
      const { parent } = place();
      const r = await c.api.importIssues(
        { repo: repo.trim(), label: label.trim(), parent, dry_run: dry },
        { store: store() },
      );
      if (dry) {
        counted = r;
      } else {
        toast({ message: `${r.created.length} 件を取り込みました` });
        counted = null;
        done();
      }
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  return {
    get repo() {
      return repo;
    },
    set repo(v) {
      repo = v;
      counted = null;
    },
    get label() {
      return label;
    },
    set label(v) {
      label = v;
      counted = null;
    },
    get counted() {
      return counted;
    },
    get busy() {
      return busy;
    },
    get error() {
      return error;
    },
    get ready() {
      return ready;
    },
    get cli() {
      return cli;
    },
    forget: () => {
      counted = null;
    },
    run,
  };
}
