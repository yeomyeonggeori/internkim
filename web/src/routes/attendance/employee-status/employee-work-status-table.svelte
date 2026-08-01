<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Table from '$lib/components/ui/table';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { AttendanceEmployeeWorkStatus } from '../attendance-api';
	import type { AttendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';

	type Props = {
		employees: AttendanceEmployeeWorkStatus[];
		text: AttendanceText;
		statusLabel: (status: string) => string;
		onSelect: (employee: AttendanceEmployeeWorkStatus) => void;
	};

	let { employees, text, statusLabel, onSelect }: Props = $props();

	function formatDifference(minutes: number): string {
		const prefix = minutes > 0 ? '+' : minutes < 0 ? '-' : '';
		return `${prefix}${formatWorkStatusDuration(Math.abs(minutes), text)}`;
	}

	function barScale(employee: AttendanceEmployeeWorkStatus): number {
		return Math.max(employee.targetMinutes, displayedActualMinutes(employee), 1);
	}

	function displayedActualMinutes(employee: AttendanceEmployeeWorkStatus): number {
		return Math.max(0, employee.actualMinutes) + Math.max(0, employee.provisionalMinutes);
	}

	function actualBarWidth(employee: AttendanceEmployeeWorkStatus): number {
		return (displayedActualMinutes(employee) / barScale(employee)) * 100;
	}
</script>

<Table.Root class="min-w-[77rem] table-fixed" data-testid="employee-work-status-table">
	<colgroup>
		<col class="w-60" />
		<col class="w-24" />
		<col class="w-44" />
		<col class="w-28" />
		<col class="w-48" />
		<col class="w-28" />
		<col class="w-28" />
		<col class="w-36" />
		<col class="w-12" />
	</colgroup>
	<Table.Header>
		<Table.Row>
			<Table.Head class="text-left">{text.workStatus.employee}</Table.Head>
			<Table.Head class="text-left">{text.workStatus.mode}</Table.Head>
			<Table.Head class="text-left">{text.workStatus.actual}</Table.Head>
			<Table.Head class="text-left">{text.workStatus.leave}</Table.Head>
			<Table.Head class="text-left">{text.workStatus.fulfilled}</Table.Head>
			<Table.Head class="text-left">{text.workStatus.difference}</Table.Head>
			<Table.Head class="text-left">{text.workStatus.night}</Table.Head>
			<Table.Head class="text-left">{text.status}</Table.Head>
			<Table.Head class="text-center"><span class="sr-only">{text.workStatus.details}</span></Table.Head>
		</Table.Row>
	</Table.Header>
	<Table.Body>
		{#each employees as employee (employee.email)}
			<Table.Row>
				<Table.Cell class="align-top text-left">
					<div class="flex items-center gap-2">
						<PersonAvatar
							name={employee.displayName}
							email={employee.email}
							seed={employee.email}
							class="size-8 shrink-0"
						/>
						<span class="grid">
							<span class="font-medium">{employee.displayName}</span>
							<span class="text-xs text-muted-foreground">{employee.email}</span>
						</span>
					</div>
				</Table.Cell>
				<Table.Cell class="align-top text-left">{text.workStatus[employee.workMode]}</Table.Cell>
				<Table.Cell class="align-top text-left">
					<div class="grid gap-1">
						<span
							class="block tabular-nums"
							data-testid="employee-work-status-metric"
							aria-label={`${text.workStatus.actual} ${formatWorkStatusDuration(employee.actualMinutes, text)}, ${text.workStatus.provisional} ${formatWorkStatusDuration(employee.provisionalMinutes, text)}`}
						>
							{formatWorkStatusDuration(displayedActualMinutes(employee), text)}
						</span>
						<div class="flex h-1.5 overflow-hidden rounded-full bg-muted">
							<div class="bg-yellow-400" style={`width:${actualBarWidth(employee)}%`}></div>
						</div>
					</div>
				</Table.Cell>
				<Table.Cell class="align-top text-left">
					<span class="block tabular-nums" data-testid="employee-work-status-metric">
						{formatWorkStatusDuration(employee.leaveMinutes, text)}
					</span>
				</Table.Cell>
				<Table.Cell class="align-top text-left">
					<span class="block tabular-nums" data-testid="employee-work-status-metric">
						{#if employee.hasBaseline}
							{formatWorkStatusDuration(employee.fulfilledMinutes, text)}
							<span class="text-muted-foreground">
								/ {formatWorkStatusDuration(employee.targetMinutes, text)}
							</span>
						{:else}
							{text.workStatus.noBaseline}
						{/if}
					</span>
				</Table.Cell>
				<Table.Cell class="align-top text-left">
					<span class="block tabular-nums" data-testid="employee-work-status-metric">
						{employee.hasBaseline
							? formatDifference(employee.differenceMinutes)
							: text.workStatus.noBaseline}
					</span>
				</Table.Cell>
				<Table.Cell class="align-top text-left">
					<span class="block tabular-nums" data-testid="employee-work-status-metric">
						{formatWorkStatusDuration(employee.nightMinutes, text)}
					</span>
				</Table.Cell>
				<Table.Cell class="align-top text-left"><Badge variant="secondary">{statusLabel(employee.status)}</Badge></Table.Cell>
				<Table.Cell class="align-top text-center">
					<Button
						variant="ghost"
						size="icon-sm"
						aria-label={`${employee.displayName} ${text.workStatus.details}`}
						onclick={() => onSelect(employee)}
					>
						<ChevronRightIcon />
					</Button>
				</Table.Cell>
			</Table.Row>
		{:else}
			<Table.Row>
				<Table.Cell colspan={9} class="h-28 text-center text-muted-foreground">
					{text.workStatus.noEmployees}
				</Table.Cell>
			</Table.Row>
		{/each}
	</Table.Body>
</Table.Root>
