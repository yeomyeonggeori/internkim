<script lang="ts">
	import { tick } from 'svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import { flowDefinitionPaletteColor } from '../flow-definition-colors';
	import type { FlowMember } from '../flow-types';
	import type { FlowMemberScoreSection } from './flow-report-data';
	import { memberScoreScale, memberScoreWidth } from './flow-member-score-scale';

	type Props = {
		section: FlowMemberScoreSection;
		members?: FlowMember[];
	};

	let { section, members = [] }: Props = $props();
	let rowScrollViewport = $state<HTMLElement | null>(null);
	let hasRowScrollOverflow = $state(false);
	let isRowScrollAtEnd = $state(true);

	const rowScrollFadeThreshold = 16;
	const barColor = flowDefinitionPaletteColor(0);
	const scale = $derived(memberScoreScale(section.rows.map((row) => row.total)));
	const averageWidth = $derived(memberScoreWidth(section.averageValue, scale));

	$effect(() => {
		section.rows.length;
		tick().then(updateRowScrollFade);
	});

	function isTopScore(value: number): boolean {
		return value > 0 && value === scale.highestValue;
	}

	function memberOf(name: string): FlowMember | undefined {
		return members.find((member) => member.name === name);
	}

	function formatValue(value: number, unit: string): string {
		return `${value}${unit}`;
	}

	function memberScrollHint(itemCount: number): string {
		return section.memberScrollHint.replace('{count}', String(itemCount));
	}

	function updateRowScrollFade(): void {
		if (!rowScrollViewport) {
			hasRowScrollOverflow = false;
			isRowScrollAtEnd = true;
			return;
		}

		const overflowAmount = rowScrollViewport.scrollHeight - rowScrollViewport.clientHeight;
		const scrollRemaining = overflowAmount - rowScrollViewport.scrollTop;
		hasRowScrollOverflow = overflowAmount > rowScrollFadeThreshold;
		isRowScrollAtEnd = scrollRemaining <= 2;
	}

	function shouldShowRowScrollFade(): boolean {
		return hasRowScrollOverflow && !isRowScrollAtEnd;
	}
</script>

<div class="-mt-1 flex h-full min-h-0 flex-col gap-1.5">
	<div class="relative min-h-0 flex-1">
		<div
			bind:this={rowScrollViewport}
			class="divide-border/50 h-full min-h-28 divide-y overflow-y-auto pb-8 pr-1"
			onscroll={updateRowScrollFade}
		>
			{#each section.rows as row (row.label)}
				{@const member = memberOf(row.label)}
				<div class="flex items-center gap-3 py-2">
					<PersonAvatar
						name={row.label}
						email={member?.email ?? ''}
						seed={member?.id ?? row.label}
						image={personProfileImagePath(member?.id)}
						class="size-7 shrink-0"
					/>
					<div class="min-w-0 flex-1 space-y-1">
						<div class="flex items-baseline justify-between gap-3">
							<span class="min-w-0 truncate text-sm font-medium">{row.label}</span>
							<span
								class={isTopScore(row.total)
									? 'shrink-0 text-sm font-semibold tabular-nums text-blue-600'
									: 'text-foreground shrink-0 text-sm tabular-nums'}
							>
								{formatValue(row.total, section.unit)}
							</span>
						</div>
						<div class="bg-muted relative h-1.5 overflow-hidden rounded-full">
							<div
								class="h-full rounded-full"
								style={`width: ${memberScoreWidth(row.total, scale)}%; background: ${barColor}`}
								aria-label={`${row.label} ${formatValue(row.total, section.unit)}`}
							></div>
							{#if section.averageValue > 0}
								<span
									aria-hidden="true"
									class="bg-foreground/40 absolute inset-y-0 w-px"
									style={`left: ${averageWidth}%`}
								></span>
							{/if}
						</div>
						{#if row.summary}
							<p class="text-muted-foreground truncate text-[11px] leading-none">{row.summary}</p>
						{/if}
					</div>
				</div>
			{/each}
		</div>
		{#if shouldShowRowScrollFade()}
			<div class="from-card via-card/75 pointer-events-none absolute inset-x-0 bottom-0 h-8 rounded-b-md bg-gradient-to-t to-transparent"></div>
		{/if}
	</div>
	{#if section.rows.length > 4}
		<div class="text-muted-foreground text-xs">{memberScrollHint(section.rows.length)}</div>
	{/if}
</div>
