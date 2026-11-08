<script lang="ts" module>
  export type IconName =
    | "search"
    | "settings"
    | "sync"
    | "close"
    | "more"
    | "chevron-right"
    | "chevron-down"
    | "plus"
    | "check"
    | "copy"
    | "lock"
    | "eye-off"
    | "wifi-off"
    | "warning"
    | "spinner"
    | "folder"
    | "folder-plus"
    | "workspace"
    | "filter"
    | "forbid"
    | "checklist"
    | "status"
    | "undo"
    | "bell"
    | "activity"
    | "people"
    | "database"
    | "grip";

  type Def = { box: 16 | 24; w: number; d: string };

  // Paths are the ones of the artboards: a 24 grid with a 2 stroke, a 16 grid with a 1.4 stroke.
  const icons: Record<IconName, Def> = {
    search: { box: 24, w: 2, d: "M18 11a7 7 0 1 1-14 0a7 7 0 0 1 14 0zM20 20l-3.5-3.5" },
    settings: {
      box: 24,
      w: 2,
      d: "M15 12a3 3 0 1 1-6 0a3 3 0 0 1 6 0zM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z",
    },
    sync: {
      box: 24,
      w: 2,
      d: "M20 12a8 8 0 0 1-14.3 4.9M4 12a8 8 0 0 1 14.3-4.9M18 3v4h-4M6 21v-4h4",
    },
    close: { box: 24, w: 2.2, d: "M6 6l12 12M18 6L6 18" },
    more: { box: 24, w: 3.4, d: "M5 12h.01M12 12h.01M19 12h.01" },
    "chevron-right": { box: 24, w: 2, d: "M9 6l6 6-6 6" },
    "chevron-down": { box: 24, w: 2, d: "M6 9l6 6 6-6" },
    plus: { box: 24, w: 2, d: "M12 5v14M5 12h14" },
    check: { box: 24, w: 2.4, d: "M5 12l5 5L20 7" },
    copy: {
      box: 24,
      w: 2,
      d: "M11 9h7a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2h-7a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2zM5 15V6a2 2 0 0 1 2-2h9",
    },
    lock: {
      box: 24,
      w: 2.2,
      d: "M7 11h10a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-6a2 2 0 0 1 2-2zM8 11V7a4 4 0 0 1 8 0v4",
    },
    "eye-off": {
      box: 24,
      w: 2,
      d: "M2 2l20 20M10.6 6.1A9 9 0 0 1 12 6c5 0 9 6 9 6a15 15 0 0 1-3.2 3.8M6.5 7.7C3.8 9.5 3 12 3 12s4 6 9 6a8 8 0 0 0 3.2-.7M9.9 9.9a3 3 0 0 0 4.2 4.2",
    },
    "wifi-off": {
      box: 24,
      w: 2,
      d: "M2 2l20 20M8.5 16.5a5 5 0 0 1 7 0M5 13a10 10 0 0 1 5.2-2.8M19 13a10 10 0 0 0-2.2-1.6M2 8.8a15 15 0 0 1 4.4-2.6M22 8.8A15 15 0 0 0 10.7 5M12 20h.01",
    },
    warning: { box: 24, w: 2, d: "M12 4l9 16H3zM12 10v4M12 17.5h.01" },
    spinner: { box: 24, w: 2.6, d: "M12 3a9 9 0 1 0 9 9" },
    folder: { box: 16, w: 1.4, d: "M2.5 4.5h4l1.5 1.5h5.5v6.5h-11z" },
    "folder-plus": {
      box: 16,
      w: 1.4,
      d: "M2.5 4.5h4l1.5 1.5h5.5v6.5h-11zM8 7.5v3.5M6.25 9.25h3.5",
    },
    workspace: { box: 16, w: 1.4, d: "M2.5 3.5h11v9h-11zM2.5 6.5h11" },
    filter: { box: 24, w: 2, d: "M3 5h18l-7 8v6l-4-2v-4z" },
    forbid: { box: 16, w: 1.4, d: "M8 2.5a5.5 5.5 0 1 1 0 11a5.5 5.5 0 0 1 0-11zM4.1 4.1l7.8 7.8" },
    checklist: { box: 24, w: 2, d: "M4 6l2 2 3-3.5M4 14l2 2 3-3.5M13 7h7M13 15h7" },
    bell: {
      box: 24,
      w: 2,
      d: "M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0",
    },
    activity: { box: 24, w: 2, d: "M3 12h4l3-8 4 16 3-8h4" },
    people: {
      box: 24,
      w: 2,
      d: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8",
    },
    database: {
      box: 24,
      w: 2,
      d: "M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3zM4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3",
    },
    grip: { box: 24, w: 3.4, d: "M9 5h.01M9 12h.01M9 19h.01M15 5h.01M15 12h.01M15 19h.01" },
    undo: { box: 24, w: 2, d: "M9 14L4 9l5-5M4 9h10.5a5.5 5.5 0 0 1 0 11H11" },
    status: {
      box: 16,
      w: 1.4,
      d: "M8 2.5a5.5 5.5 0 1 1 0 11a5.5 5.5 0 0 1 0-11zM8 6a2 2 0 1 1 0 4a2 2 0 0 1 0-4z",
    },
  };
</script>

<script lang="ts">
  type Props = { name: IconName; size?: number; label?: string };
  let { name, size = 16, label }: Props = $props();
  const def = $derived(icons[name]);
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 {def.box} {def.box}"
  fill="none"
  stroke="currentColor"
  stroke-width={def.w}
  stroke-linecap="round"
  stroke-linejoin="round"
  role={label ? "img" : undefined}
  aria-label={label}
  aria-hidden={label ? undefined : "true"}
>
  <path d={def.d} />
</svg>
