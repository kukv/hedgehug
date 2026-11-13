export {
  createLinkDialog,
  type LinkDialog,
  type LinkDialogState,
} from "./features/github/linkDialog";
export { createLinkList, type LinkList, type LinkListState } from "./features/github/links";
export {
  createProjectLinks,
  type ProjectLinks,
  type ProjectLinksState,
} from "./features/github/projectLinks";
export {
  createRepoFailures,
  type RepoFailures,
  type RepoFailuresState,
  scanFailedText,
} from "./features/github/repoFailures";
export {
  checksLabel,
  kindLabel,
  linkDialogTitle,
  linkText,
  linkUrl,
  markerCopiedMessage,
  markerCopyFailedMessage,
  markerOf,
  noAccountNote,
  noLinksText,
  reviewLabel,
  showsProgress,
  stateLabel,
  unlinkedMessage,
} from "./features/github/text";
export type {
  LinkChecks,
  LinkInput,
  LinkKind,
  LinkReview,
  LinkRow,
  LinkState,
  LinkTarget,
  ProjectLinkRow,
  RepoFailure,
} from "./model/link";
export type { LinksPort } from "./port/links";
