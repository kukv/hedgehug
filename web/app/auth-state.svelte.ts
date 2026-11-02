let lost = $state(false);

/** Whether the server refused the token (401 auth.token); App shows the NoToken screen. */
export const auth = {
  get lost(): boolean {
    return lost;
  },
  markLost(): void {
    lost = true;
  },
  /** For tests: forgets that the token was refused. */
  reset(): void {
    lost = false;
  },
};
