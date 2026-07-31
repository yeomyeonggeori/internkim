<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { mergeProps } from 'bits-ui';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';

	type Props = {
		actualMinutes: number;
		creditedLeaveMinutes: number;
		capacityMinutes: number;
		hasBaseline: boolean;
		targetMinutes: number;
	};

	let {
		actualMinutes,
		creditedLeaveMinutes,
		capacityMinutes,
		hasBaseline,
		targetMinutes
	}: Props = $props();
	const text = createPageText(attendanceText);
	const isMobile = new IsMobile();
	const actualBarMinutes = $derived(
		Math.min(Math.max(actualMinutes, 0), capacityMinutes)
	);
	const leaveBarMinutes = $derived(
		Math.min(
			Math.max(creditedLeaveMinutes, 0),
			Math.max(capacityMinutes - actualBarMinutes, 0)
		)
	);
	const unusedMinutes = $derived(
		Math.max(capacityMinutes - actualBarMinutes - leaveBarMinutes, 0)
	);
	const actualWidth = $derived((actualBarMinutes / capacityMinutes) * 100);
	const leaveWidth = $derived((leaveBarMinutes / capacityMinutes) * 100);
	const remainingWidth = $derived((unusedMinutes / capacityMinutes) * 100);
	const targetPosition = $derived(
		hasBaseline
			? (Math.min(targetMinutes, capacityMinutes) / capacityMinutes) * 100
			: 0
	);
	const barLabel = $derived(
		`${text.workStatus.totalCapacity} ${formatWorkStatusDuration(capacityMinutes, text)}, ${text.workStatus.actual} ${formatWorkStatusDuration(actualMinutes, text)}, ${text.workStatus.creditedLeave} ${formatWorkStatusDuration(creditedLeaveMinutes, text)}, ${text.workStatus.remaining} ${formatWorkStatusDuration(unusedMinutes, text)}${hasBaseline ? `, ${text.workStatus.target} ${formatWorkStatusDuration(targetMinutes, text)}` : ''}`
	);
</script>

{#snippet BarTrigger({ props }: { props?: Record<string, unknown> })}
	{@const mergedProps = mergeProps(
		{
			class:
				'relative block w-full cursor-default rounded-sm pt-5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring',
			'aria-label': barLabel,
			'data-testid': 'work-standard-capacity-bar'
		},
		props ?? {}
	)}
	<button type="button" {...mergedProps}>
		{#if hasBaseline}
			<span
				class="absolute top-0 -translate-x-1/2 whitespace-nowrap text-[9px] font-medium text-foreground"
				style={`left:${targetPosition}%`}
				aria-hidden="true"
			>
				{text.workStatus.target} {formatWorkStatusDuration(targetMinutes, text)}
			</span>
		{/if}
		<span class="relative flex h-2.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
			<span class="bg-foreground" style={`width:${actualWidth}%`}></span>
			<span class="bg-blue-500" style={`width:${leaveWidth}%`}></span>
			<span class="bg-muted-foreground/20" style={`width:${remainingWidth}%`}></span>
			{#if hasBaseline}
				<span
					class="absolute inset-y-0 w-0.5 bg-foreground"
					style={`left:${targetPosition}%`}
					data-testid="work-standard-target-marker"
				></span>
			{/if}
		</span>
		<span
			class="mt-1 flex justify-between text-[9px] text-muted-foreground tabular-nums"
			aria-hidden="true"
		>
			<span>{formatWorkStatusDuration(0, text)}</span>
			<span>{formatWorkStatusDuration(capacityMinutes, text)}</span>
		</span>
	</button>
{/snippet}

{#snippet Breakdown()}
	<div class="grid min-w-52 gap-2" data-testid="work-standard-bar-tooltip">
		<div class="flex items-center justify-between gap-5">
			<span class="flex items-center gap-2">
				<i class="inline-block size-2.5 rounded-sm bg-foreground"></i>
				{text.workStatus.actual}
			</span>
			<span class="font-medium tabular-nums">{formatWorkStatusDuration(actualMinutes, text)}</span>
		</div>
		<div class="flex items-center justify-between gap-5">
			<span class="flex items-center gap-2">
				<i class="inline-block size-2.5 rounded-sm bg-blue-500"></i>
				{text.workStatus.creditedLeave}
			</span>
			<span class="font-medium tabular-nums">{formatWorkStatusDuration(creditedLeaveMinutes, text)}</span>
		</div>
		<div class="flex items-center justify-between gap-5">
			<span class="flex items-center gap-2">
				<i class="inline-block size-2.5 rounded-sm bg-muted-foreground/20"></i>
				{text.workStatus.remaining}
			</span>
			<span class="font-medium tabular-nums">{formatWorkStatusDuration(unusedMinutes, text)}</span>
		</div>
		{#if hasBaseline}
			<div class="flex items-center justify-between gap-5 border-t pt-2">
				<span class="text-muted-foreground">{text.workStatus.target}</span>
				<span class="font-medium tabular-nums">{formatWorkStatusDuration(targetMinutes, text)}</span>
			</div>
		{/if}
	</div>
{/snippet}

{#if isMobile.current}
	<Popover.Root>
		<Popover.Trigger>
			{#snippet child({ props })}
				{@render BarTrigger({ props })}
			{/snippet}
		</Popover.Trigger>
		<Popover.Content side="top" sideOffset={8} class="w-auto max-w-64 p-3">
			{@render Breakdown()}
		</Popover.Content>
	</Popover.Root>
{:else}
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				{@render BarTrigger({ props })}
			{/snippet}
		</Tooltip.Trigger>
		<Tooltip.Content
			side="top"
			sideOffset={8}
			class="mx-2 grid max-w-64 gap-2 bg-popover px-3 py-2 text-popover-foreground shadow-md ring-1 ring-border"
			arrowClasses="bg-popover fill-popover"
		>
			{@render Breakdown()}
		</Tooltip.Content>
	</Tooltip.Root>
{/if}

<div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
	<span><i class="mr-1 inline-block size-2 rounded-full bg-foreground"></i>{text.workStatus.actual}</span>
	<span><i class="mr-1 inline-block size-2 rounded-full bg-blue-500"></i>{text.workStatus.creditedLeave}</span>
	<span><i class="mr-1 inline-block size-2 rounded-full bg-muted-foreground/20"></i>{text.workStatus.remaining}</span>
</div>
