<script lang="ts">
	import { contributionTotal, contributionWeeks, type CompanyActivity } from './activity';
	import { dataRoomCompanyText } from './company-text';

	let { activity, locale }: { activity: CompanyActivity; locale: 'ko' | 'en' } = $props();

	const weekCount = 53;
	const levels = [0, 1, 2, 3, 4];
	const text = $derived(dataRoomCompanyText[locale].activity);
	const weeks = $derived(contributionWeeks(activity, weekCount));
	const total = $derived(contributionTotal(activity));

	function cellLabel(date: string, count: number): string {
		return `${date} · ${text.events(count)}`;
	}
</script>

<div class="grid gap-3">
	<div class="overflow-x-auto [direction:rtl] max-sm:[mask-image:linear-gradient(to_right,transparent,black_2.5rem)]">
		<div class="grid min-w-[42rem] grid-cols-[1.5rem_minmax(0,1fr)] [direction:ltr] gap-x-2 gap-y-1.5 text-[0.625rem] leading-none">
			<span></span>
			<div class="grid gap-[3px]" style:grid-template-columns={`repeat(${weekCount}, minmax(0, 1fr))`} aria-hidden="true">
				{#each weeks as week, index (index)}
					<span class="text-muted-foreground overflow-visible whitespace-nowrap">{week.month ? text.month(Number(week.month)) : ''}</span>
				{/each}
			</div>
			<div class="text-muted-foreground grid grid-rows-7 gap-[3px]" aria-hidden="true">
				{#each text.weekdays as weekday, index (index)}
					<span class="flex items-center">{weekday}</span>
				{/each}
			</div>
			<div class="grid gap-[3px]" style:grid-template-columns={`repeat(${weekCount}, minmax(0, 1fr))`} role="img" aria-label={text.contributionsLabel(total)}>
				{#each weeks as week, index (index)}
					<div class="grid grid-rows-7 gap-[3px]">
						{#each week.days as day, dayIndex (dayIndex)}
							{#if day}
								<span class="contribution-cell aspect-square rounded-[2px]" data-level={day.level} title={cellLabel(day.date, day.count)}></span>
							{:else}
								<span class="aspect-square"></span>
							{/if}
						{/each}
					</div>
				{/each}
			</div>
		</div>
	</div>
	<div class="text-muted-foreground flex flex-wrap items-center justify-end gap-2 text-xs">
		<span class="inline-flex items-center gap-1" aria-hidden="true">
			{text.less}
			{#each levels as level (level)}<span class="contribution-cell size-2.5 rounded-[2px]" data-level={level}></span>{/each}
			{text.more}
		</span>
	</div>
</div>

<style>
	.contribution-cell {
		background-color: var(--color-muted);
		box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-foreground) 6%, transparent);
	}
	.contribution-cell[data-level='1'] { background-color: color-mix(in oklch, var(--color-foreground) 12%, var(--color-muted)); }
	.contribution-cell[data-level='2'] { background-color: color-mix(in oklch, var(--color-foreground) 24%, var(--color-muted)); }
	.contribution-cell[data-level='3'] { background-color: color-mix(in oklch, var(--color-foreground) 40%, var(--color-muted)); }
	.contribution-cell[data-level='4'] { background-color: color-mix(in oklch, var(--color-foreground) 62%, var(--color-muted)); }
</style>
