import { taskStatus } from '$lib/task/central-task';
import { taskSizeNames } from '$lib/task/task-sizes';

export const WorkspaceTaskStatus = taskStatus;

export const WorkspaceTaskInitialStatus = {
  planned: taskStatus.planned,
  inProgress: taskStatus.inProgress,
  completed: taskStatus.completed,
  paused: taskStatus.paused,
  rejected: taskStatus.rejected,
  stopped: taskStatus.stopped,
} as const;

export const WorkspaceTaskSize = {
  ExtraSmall: taskSizeNames[0],
  Small: taskSizeNames[1],
  Medium: taskSizeNames[2],
  Large: taskSizeNames[3],
  ExtraLarge: taskSizeNames[4],
  ExtraExtraLarge: taskSizeNames[5],
} as const;

export enum WorkspaceTaskScope {
  Self = 'self',
  All = 'all',
}
