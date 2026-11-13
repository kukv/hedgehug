import type { Project, ProjectsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  type Project as ApiProject,
  addProjectRepo,
  createProject,
  getProject,
  removeProjectRepo,
  renameProject,
  setProjectDate,
  setProjectOwner,
  setProjectStatus,
} from "./generated";

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
    repos: p.repos ?? [],
  };
}

export function createProjectsPort(): ProjectsPort {
  return {
    get: (storeId, projectId) =>
      call(async () => toProject(storeId, await getProject(storeId, projectId))),
    create: (storeId, workspaceId, input) =>
      call(async () => toProject(storeId, await createProject(storeId, workspaceId, input))),
    rename: (storeId, projectId, title) =>
      call(async () => toProject(storeId, await renameProject(storeId, projectId, { title }))),
    setStatus: (storeId, projectId, status) =>
      call(async () => toProject(storeId, await setProjectStatus(storeId, projectId, { status }))),
    setOwner: (storeId, projectId, owner) =>
      call(async () => toProject(storeId, await setProjectOwner(storeId, projectId, { owner }))),
    setDate: (storeId, projectId, field, value) =>
      call(async () =>
        toProject(storeId, await setProjectDate(storeId, projectId, { field, value })),
      ),
    addRepo: (storeId, projectId, repo) =>
      call(async () => toProject(storeId, await addProjectRepo(storeId, projectId, { repo }))),
    removeRepo: (storeId, projectId, repo) =>
      call(async () => toProject(storeId, await removeProjectRepo(storeId, projectId, { repo }))),
  };
}
