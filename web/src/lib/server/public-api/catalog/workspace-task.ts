import { taskStatus } from '$lib/task/central-task';

export const WorkspaceTaskStatus = taskStatus;

export const WorkspaceTaskInitialStatus = {
  planned: taskStatus.planned,
  inProgress: taskStatus.inProgress,
  completed: taskStatus.completed,
  paused: taskStatus.paused,
  rejected: taskStatus.rejected,
  stopped: taskStatus.stopped,
} as const;

export enum WorkspaceTaskSize {
  ExtraSmall = 'XS',
  Small = 'S',
  Medium = 'M',
  Large = 'L',
  ExtraLarge = 'XL',
  ExtraExtraLarge = 'XXL',
}

export enum WorkspaceTaskScope {
  Self = 'self',
  All = 'all',
}
