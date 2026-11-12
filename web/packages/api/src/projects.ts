import type { Project, ProjectsPort } from "@hedgehug/core";
import { call } from "./errors";
import { type Project as ApiProject, createProject, getProject } from "./generated";

export function toProject(storeId: string, p: ApiProject): Project {
  return {
    id: p.id,
    storeId,
    workspaceId: p.workspace,
    number: p.number,
    rank: p.rank,
    title: p.title,
    owner: p.owner,
    status: p.status,
    start: p.start,
    end: p.end,
  };
}

export function createProjectsPort(): ProjectsPort {
  return {
    get: (storeId, projectId) =>
      call(async () => toProject(storeId, await getProject(storeId, projectId))),
    create: (storeId, workspaceId, input) =>
      call(async () => toProject(storeId, await createProject(storeId, workspaceId, input))),
  };
}
