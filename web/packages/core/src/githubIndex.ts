export { createLinkList, type LinkList, type LinkListState } from "./features/github/links";
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
} from "./model/link";
export { linkKey } from "./model/link";
export type { LinksPort } from "./port/links";
