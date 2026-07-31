<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import type { AttendanceEmployeeWorkStatus } from '../attendance-api';
	import type { AttendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';

	type Props = {
		employee: AttendanceEmployeeWorkStatus | null;
		text: AttendanceText;
		statusLabel: (status: string) => string;
		onClose: () => void;
	};

	let { employee, text, statusLabel, onClose }: Props = $props();
</script>

<Dialog.Root open={employee !== null} onOpenChange={(open) => !open && onClose()}>
	<Dialog.Content class="max-h-[85vh] overflow-y-auto sm:max-w-3xl" closeLabel={text.close}>
		{#if employee}
			<Dialog.Header>
				<Dialog.Title>{employee.displayName} · {text.workStatus.dailyBreakdown}</Dialog.Title>
				<Dialog.Description>{employee.periodStart}–{employee.periodEnd}</Dialog.Description>
			</Dialog.Header>
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>{text.date}</Table.Head>
						<Table.Head>{text.status}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.target}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.actual}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.paidLeave}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.overtime}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.night}</Table.Head>
						<Table.Head>{text.workStatus.workSegments}</Table.Head>
						<Table.Head>{text.workStatus.leaveSegments}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each employee.days as day (day.date)}
						<Table.Row>
							<Table.Cell>{day.date}</Table.Cell>
							<Table.Cell><Badge variant="secondary">{statusLabel(day.status)}</Badge></Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.targetMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.actualMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.paidLeaveMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.overtimeMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.nightMinutes, text)}</Table.Cell>
							<Table.Cell>
								{#if day.workSegments.length > 0}
									{day.workSegments
										.map((segment) => `${segment.startTime}–${segment.endTime}${segment.provisional ? ` (${text.workStatus.statusWorking})` : ''}`)
										.join(', ')}
								{:else}
									{text.workStatus.noSegments}
								{/if}
							</Table.Cell>
							<Table.Cell>
								{#if day.leaveSegments.length > 0}
									{day.leaveSegments
										.map((segment) => `${segment.startTime}–${segment.endTime} (${segment.paid ? text.workStatus.paid : text.workStatus.unpaid})`)
										.join(', ')}
								{:else}
									{text.workStatus.noSegments}
								{/if}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
			<Dialog.Footer>
				<Button variant="outline" onclick={onClose}>{text.workStatus.close}</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
