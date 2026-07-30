<script lang="ts">
	import { tick } from 'svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type { AdminPageText, AttendanceLeavePolicy, LeaveAllowedUnit, LeaveType } from './admin-types';

	type Props = {
		policy: AttendanceLeavePolicy | null;
		pendingDraft: LeaveType | null;
		selectedID: string;
		message: string;
		isLoading: boolean;
		isSaving: boolean;
		text: AdminPageText;
		onAdd: () => void;
		onSelect: (leaveType: LeaveType) => void;
	};

	let {
		policy,
		pendingDraft,
		selectedID,
		message,
		isLoading,
		isSaving,
		text,
		onAdd,
		onSelect
	}: Props = $props();
	let scrollViewport = $state<HTMLElement | null>(null);
	let hasScrollOverflow = $state(false);
	let isScrollAtEnd = $state(true);

	const scrollFadeThreshold = 16;

	$effect(() => {
		policy?.leaveTypes.length;
		pendingDraft !== null;
		tick().then(updateScrollFade);
	});

	function updateScrollFade(): void {
		if (!scrollViewport) {
			hasScrollOverflow = false;
			isScrollAtEnd = true;
			return;
		}
		const overflowAmount = scrollViewport.scrollHeight - scrollViewport.clientHeight;
		const scrollRemaining = overflowAmount - scrollViewport.scrollTop;
		hasScrollOverflow = overflowAmount > scrollFadeThreshold;
		isScrollAtEnd = scrollRemaining <= 2;
	}

	function shouldShowScrollFade(): boolean {
		return hasScrollOverflow && !isScrollAtEnd;
	}

	function paidLabel(leaveType: LeaveType): string {
		return leaveType.paid ? text.attendanceSettings.paid : text.attendanceSettings.unpaid;
	}

	function leaveTypeName(leaveType: LeaveType): string {
		return localizedLeaveTypeName(leaveType.id, leaveType.name, currentLocale.value);
	}

	function allowedUnitLabel(unit: LeaveAllowedUnit): string {
		if (unit === 'fullDay') return text.attendanceSettings.fullDay;
		if (unit === 'halfDay') return text.attendanceSettings.halfDay;
		return text.attendanceSettings.quarterDay;
	}
</script>

<svelte:window onresize={updateScrollFade} />

<Card.Root class="min-h-0 lg:h-full">
	<Card.Header>
		<Card.Title>{text.attendanceSettings.title}</Card.Title>
		<Card.Description>{text.attendanceSettings.description}</Card.Description>
	</Card.Header>
	<Card.Content class="flex min-h-0 flex-1 flex-col gap-4">
		{#if isLoading}
			<p class="text-sm text-muted-foreground">{text.attendanceSettings.loading}</p>
		{:else if policy}
			<div class="relative min-h-0 lg:flex-1">
				<div
					bind:this={scrollViewport}
					class="grid max-h-[min(45vh,42rem)] gap-2 overflow-y-auto overscroll-contain pb-8 pr-1 sm:max-h-[min(65vh,42rem)] lg:h-full lg:max-h-none"
					data-testid="leave-policy-list-scroll"
					onscroll={updateScrollFade}
				>
					{#each policy.leaveTypes.filter((leaveType) => leaveType.isActive) as leaveType (leaveType.id)}
						<button
							type="button"
							class="flex items-center justify-between rounded-md border p-3 text-left hover:bg-muted"
							class:border-primary={selectedID === leaveType.id}
							onclick={() => onSelect(leaveType)}
						>
							<span>
								<span class="block font-medium">{leaveTypeName(leaveType)}</span>
								<span class="text-xs text-muted-foreground">
									{paidLabel(leaveType)} · {leaveType.allowedUnits
										.map(allowedUnitLabel)
										.join(', ')}
								</span>
							</span>
						</button>
					{/each}
					{#if pendingDraft}
						<div
							class="flex items-center justify-between rounded-md border border-primary bg-muted/40 p-3 text-left"
							data-testid="pending-leave-type"
						>
							<span>
								<span class="block font-medium">
									{pendingDraft.name.trim() || text.attendanceSettings.newLeave}
								</span>
								<span class="text-xs text-muted-foreground">
									{paidLabel(pendingDraft)} · {pendingDraft.allowedUnits
										.map(allowedUnitLabel)
										.join(', ')}
								</span>
							</span>
							<Badge variant="secondary">{text.attendanceSettings.unsaved}</Badge>
						</div>
					{/if}
				</div>
				{#if shouldShowScrollFade()}
					<div
						class="pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-gradient-to-t from-card via-card/75 to-transparent"
						data-testid="leave-policy-list-scroll-fade"
					></div>
				{/if}
			</div>
		{/if}
		<Button variant="outline" onclick={onAdd} disabled={isLoading || isSaving || !!pendingDraft}>
			{text.attendanceSettings.add}
		</Button>
		{#if message}
			<p class="text-sm text-muted-foreground" role="status">{message}</p>
		{/if}
	</Card.Content>
</Card.Root>
