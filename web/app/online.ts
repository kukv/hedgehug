import { createSubscriber } from "svelte/reactivity";

const subscribe = createSubscriber((update) => {
  window.addEventListener("online", update);
  window.addEventListener("offline", update);
  return () => {
    window.removeEventListener("online", update);
    window.removeEventListener("offline", update);
  };
});

/** Whether the browser thinks it is online. */
export const online = {
  get current(): boolean {
    subscribe();
    return navigator.onLine;
  },
};
