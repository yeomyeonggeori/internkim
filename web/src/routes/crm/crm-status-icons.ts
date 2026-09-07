import ActivityIcon from '@lucide/svelte/icons/activity';
import CalendarIcon from '@lucide/svelte/icons/calendar';
import CheckIcon from '@lucide/svelte/icons/check';
import SearchIcon from '@lucide/svelte/icons/search';
import LoaderIcon from '@lucide/svelte/icons/loader';
import CircleMinusIcon from '@lucide/svelte/icons/circle-minus';
import PauseIcon from '@lucide/svelte/icons/pause';
import RocketIcon from '@lucide/svelte/icons/rocket';
import XIcon from '@lucide/svelte/icons/x';
import {
	isTaskStatusCompleted,
	isTaskStatusInProgress,
	isTaskStatusPaused,
	isTaskStatusRejected,
	isTaskStatusStopped
} from '../task/task-status';
import type { CRMOrganizationStatus } from './crm-types';

type IconComponent = typeof CheckIcon;

const accountStatusIcons: Record<CRMOrganizationStatus, IconComponent> = {
	prospect: LoaderIcon,
	active: ActivityIcon,
	paused: CircleMinusIcon
};

const dealStageIcons: Record<string, IconComponent> = {
	waiting: RocketIcon,
	in_progress: LoaderIcon,
	review: SearchIcon,
	done: CheckIcon,
	on_hold: PauseIcon,
	lost: XIcon
};

export function accountStatusIcon(status: CRMOrganizationStatus): IconComponent {
	return accountStatusIcons[status] ?? LoaderIcon;
}

export function dealStageIcon(stage: string): IconComponent {
	return dealStageIcons[stage] ?? RocketIcon;
}

