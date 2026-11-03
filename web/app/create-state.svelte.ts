/**
 * What the create dialog is asked for. `parent` is the ULID of the task the new one goes under.
 * `status` is the status it starts with (the column of a kanban); `category` is the category whose
 * default status it starts with (the column of a kanban by category), used when `status` is not one
 * of the workspace's.
 */
export type CreateRequest = {
  kind: "task" | "project";
  parent?: string;
  status?: string;
  category?: string;
};

let request = $state<CreateRequest | null>(null);

/** The create dialog that is asked for; CreateTask and CreateProject read it. */
export const createState = {
  get request(): CreateRequest | null {
    return request;
  },
};

export function openCreate(init: CreateRequest): void {
  request = init;
}

export function closeCreate(): void {
  request = null;
}
