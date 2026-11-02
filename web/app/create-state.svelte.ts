/** What the create dialog is asked for. `parent` is the ULID of the task the new one goes under. */
export type CreateRequest = { kind: "task" | "project"; parent?: string };

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
