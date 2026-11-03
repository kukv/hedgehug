// scpLike is user@host:owner/repo, the short form of ssh.
const scpLike = /^[^@/]+@([^:/]+):(.+)$/;
// An ID that is already host/owner/repo (not a URL).
const bare = /^[^/:@\s]+\.[^/:@\s]+\/[^/\s]+\/[^/\s]+$/;

const trimSlashes = (s: string) => s.replace(/^\/+|\/+$/g, "");
const trimGit = (s: string) => (s.endsWith(".git") ? s.slice(0, -4) : s);

function join(host: string, path: string): string | null {
  const parts = path.split("/");
  if (host === "" || parts.length !== 2 || parts[0] === "" || parts[1] === "") return null;
  return `${host}/${path}`.toLowerCase();
}

// repoIdOf turns a remote URL into <host>/<owner>/<repo> in lower case, without .git, a trailing slash or a
// port, as internal/app/usecase/current.RepoID does (https, http, ssh, git and user@host:path). A local path, or a URL
// without an owner and a repo, gives null. An ID already in that form is accepted as it is.
export function repoIdOf(input: string): string | null {
  const u = input.trim();
  let host = "";
  let path = "";
  const scp = scpLike.exec(u);
  if (scp && !u.includes("://")) {
    host = scp[1] ?? "";
    path = scp[2] ?? "";
  } else {
    let url: URL;
    try {
      url = new URL(u);
    } catch {
      const p = trimSlashes(trimGit(trimSlashes(u)));
      return bare.test(p) ? p.toLowerCase() : null;
    }
    if (!["https:", "http:", "ssh:", "git:"].includes(url.protocol)) return null;
    host = url.hostname;
    path = url.pathname;
  }
  return join(host, trimSlashes(trimGit(trimSlashes(path))));
}
