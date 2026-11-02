const KEY = "hedgehug.token";

// takeToken moves the token of the URL (#token=<t>, which hh ui prints) into
// sessionStorage and removes it from the address bar. It returns the token
// the tab holds, or null when there is none.
export function takeToken(loc: Location, storage: Storage, history: History): string | null {
  const t = new URLSearchParams(loc.hash.replace(/^#/, "")).get("token");
  if (!t) return getToken(storage);
  try {
    storage.setItem(KEY, t);
  } catch {
    // A storage that refuses writes still lets this page load use the token.
  }
  history.replaceState(null, "", loc.pathname + loc.search);
  return t;
}

// getToken is the token the tab holds.
export function getToken(storage: Storage): string | null {
  try {
    return storage.getItem(KEY);
  } catch {
    return null;
  }
}
