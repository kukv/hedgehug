// focusLost tells whether the focus has been dropped: a control the user was on is gone or disabled,
// and the focus is on nothing (the body). A focus the user moved elsewhere is not lost.
export function focusLost(): boolean {
  const a = document.activeElement;
  if (a === null || a === document.body || !a.isConnected) return true;
  return a instanceof HTMLButtonElement && a.disabled;
}
