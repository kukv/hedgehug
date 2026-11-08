<script lang="ts" module>
  export type StatusShape = "dashed" | "ring" | "pause" | "half" | "clock" | "check" | "cross";

  const byName: Record<string, StatusShape> = {
    backlog: "dashed",
    todo: "ring",
    on_hold: "pause",
    in_progress: "half",
    waiting: "clock",
    done: "check",
    canceled: "cross",
  };

  const byCategory: Record<string, StatusShape> = {
    todo: "ring",
    doing: "half",
    done: "check",
    canceled: "cross",
  };

  /** The default set has its own shapes; a status a store added keeps the shape of its category. */
  export function shapeOf(status: string, category?: string): StatusShape {
    return byName[status] ?? byCategory[category ?? ""] ?? "ring";
  }

  type Def = {
    r: number;
    filled: boolean;
    dash?: string;
    path?: string;
    pathFilled?: boolean;
    c: string;
    c2?: string;
  };

  // Paths and radii are the ones of the Foundations artboard (16px grid, 1.4 stroke).
  const defs: Record<StatusShape, Def> = {
    dashed: { r: 6, filled: false, dash: "2.2 2.2", c: "var(--muted-2)" },
    ring: { r: 6, filled: false, c: "var(--muted)" },
    pause: {
      r: 6,
      filled: false,
      path: "M6.5 5.6v4.8M9.5 5.6v4.8",
      c: "var(--muted-2)",
      c2: "var(--muted)",
    },
    half: {
      r: 6,
      filled: false,
      path: "M8 4.5A3.5 3.5 0 0 1 8 11.5Z",
      pathFilled: true,
      c: "var(--accent)",
    },
    clock: { r: 6, filled: false, path: "M8 5.2V8l2 1.4", c: "var(--amber)" },
    check: { r: 7, filled: true, path: "M5 8.3l2 2 4-4.5", c: "var(--success)" },
    cross: { r: 7, filled: true, path: "M5.9 5.9l4.2 4.2M10.1 5.9l-4.2 4.2", c: "var(--muted-2)" },
  };
</script>

<script lang="ts">
  import { statusColor } from "../../core/schema-edit";

  type Props = { status: string; category?: string; color?: string; size?: number };
  let { status, category, color, size = 16 }: Props = $props();

  const shape = $derived(shapeOf(status, category));
  const def = $derived(defs[shape]);
  const custom = $derived(statusColor(color));
  const c = $derived(custom ?? def.c);
  const c2 = $derived(custom ?? def.c2 ?? def.c);
  // A filled circle (done, canceled) draws its mark in the surface color.
  const pathStroke = $derived(def.pathFilled ? "none" : def.filled ? "var(--surface)" : c2);
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 16 16"
  data-shape={shape}
  style:--c={c}
  style:--fill={def.filled ? c : "none"}
  style:--path-fill={def.pathFilled ? c : "none"}
  style:--path-stroke={pathStroke}
  aria-hidden="true"
>
  <circle cx="8" cy="8" r={def.r} stroke-width="1.4" stroke-dasharray={def.dash} />
  {#if def.path}
    <path d={def.path} stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
  {/if}
</svg>

<style>
  svg {
    flex: 0 0 auto;
  }
  circle {
    fill: var(--fill);
    stroke: var(--c);
  }
  path {
    fill: var(--path-fill);
    stroke: var(--path-stroke);
  }
</style>
