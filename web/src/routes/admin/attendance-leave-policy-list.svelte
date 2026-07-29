<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import type { AdminPageText, AttendanceLeavePolicy, LeaveAllowedUnit, LeaveType } from './admin-types';

	type Props = {
		policy: AttendanceLeavePolicy | null;
		selectedID: string;
		message: string;
		isLoading: boolean;
		isSaving: boolean;
		text: AdminPageText;
		onAdd: () => void;
		onSelect: (leaveType: LeaveType) => void;
	};

	let { policy, selectedID, message, isLoading, isSaving, text, onAdd, onSelect }: Props =
		$props();

	function paidLabel(leaveType: LeaveType): string {
		return leaveType.paid ? text.attendanceSettings.paid : text.attendanceSettings.unpaid;
	}

	function allowedUnitLabel(unit: LeaveAllowedUnit): string {
		if (unit === 'fullDay') return text.attendanceSettings.fullDay;
		if (unit === 'halfDay') return text.attendanceSettings.halfDay;
		return text.attendanceSettings.quarterDay;
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.attendanceSettings.title}</Card.Title>
		<Card.Description>{text.attendanceSettings.description}</Card.Description>
	</Card.Header>
	<Card.Content class="flex flex-col gap-4">
		{#if isLoading}
			<p class="text-sm text-muted-foreground">{text.attendanceSettings.loading}</p>
		{:else if policy}
			<div class="grid gap-2">
				{#each policy.leaveTypes as leaveType (leaveType.id)}
					<button
						type="button"
						class="flex items-center justify-between rounded-md border p-3 text-left hover:bg-muted"
						class:border-primary={selectedID === leaveType.id}
						onclick={() => onSelect(leaveType)}
					>
						<span>
							<span class="block font-medium">{leaveType.name}</span>
							<span class="text-xs text-muted-foreground">
								{paidLabel(leaveType)} · {leaveType.allowedUnits.map(allowedUnitLabel).join(', ')}
							</span>
						</span>
						<Badge variant={leaveType.isActive ? 'default' : 'secondary'}>
							{leaveType.isActive ? text.attendanceSettings.active : text.attendanceSettings.inactive}
						</Badge>
					</button>
				{/each}
			</div>
		{/if}
		<Button variant="outline" onclick={onAdd} disabled={isLoading || isSaving}>
			{text.attendanceSettings.add}
		</Button>
		{#if message}
			<p class="text-sm text-muted-foreground" role="status">{message}</p>
		{/if}
	</Card.Content>
</Card.Root>
