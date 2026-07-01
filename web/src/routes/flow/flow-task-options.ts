import { buildBusinessSelectOptions } from './flow-task-workspace-model';
import { flowText } from './text';
import type { FlowDefinitions, FlowMember, FlowSummary } from './flow-types';

type FlowPageText = typeof flowText.ko;

const emptyDefinitions: FlowDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function definitionsFromSummary(summary: FlowSummary | null): FlowDefinitions {
	return summary?.definitions ?? emptyDefinitions;
}

export function statusOptionsFromSummary(summary: FlowSummary | null): string[] {
	return summary?.statusOptions ?? [];
}

export function flowTaskStatusLabel(text: FlowPageText, status: string): string {
	const labels = text.status as Record<string, string>;
	return labels[status] ?? status;
}

export function categorySelectOptions(definitions: FlowDefinitions, fallbackBusiness: string) {
	return buildBusinessSelectOptions(definitions, fallbackBusiness);
}

export function typeSelectOptions(definitions: FlowDefinitions) {
	return definitions.types.map((type) => ({ value: type, label: type }));
}

export function sizeSelectOptions(definitions: FlowDefinitions) {
	return definitions.sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
}

export function statusSelectOptions(statuses: string[], statusLabel: (status: string) => string) {
	return statuses.map((status) => ({ value: status, label: statusLabel(status) }));
}

export function memberSelectOptions(members: FlowMember[]) {
	return members.map((member) => ({ value: member.id, label: member.name }));
}
