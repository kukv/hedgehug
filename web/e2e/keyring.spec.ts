// The E2E binary keeps its secrets in a file of its own home, not in the OS keychain, and the fake
// of GitHub (github: true) is the github.com of the test:
//   - hh auth login <name> --with-token reads the token from its standard input, checks it against
//     the fake, and writes it to $HEDGEHUG_HOME/e2e-keyring.json
//   - hh auth status --json then lists the account as ok
import { existsSync } from "node:fs";
import { join } from "node:path";
import { expect, test } from "./harness";

test.use({ github: true });

test("a pasted token goes to the file keyring, not the OS keychain", ({ hh }) => {
  hh.fakegh("token", "add", "t1", "--user", "e2e");
  hh.hhIn("t1\n", "auth", "login", "e2e", "--with-token");

  expect(existsSync(join(hh.home, "hh", "e2e-keyring.json"))).toBe(true);
  const accounts = hh.json<{ name: string; token: string; state: string }[]>("auth", "status");
  const account = accounts.find((a) => a.name === "e2e");
  expect(account, JSON.stringify(accounts)).toMatchObject({ token: "keychain", state: "ok" });
});
