<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import type { QuickAddOutcome } from './task-quick-add-run';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskPageText = PageText<typeof taskText>;

	type Props = {
		outcomes: QuickAddOutcome[];
		text: TaskPageText['task'];
	};

	let { outcomes, text }: Props = $props();

	function addedDetail(outcome: Extract<QuickAddOutcome, { kind: 'added' }>): string {
		const due = outcome.endDate ? text.quickAddDue.replace('{date}', monthAndDay(outcome.endDate)) : '';
		return [outcome.ownerName, due].filter(Boolean).join(' · ');
	}

	function monthAndDay(day: string): string {
		const [, month, date] = day.split('-');
		if (!month || !date) return day;
		return `${Number(month)}/${Number(date)}`;
	}
</script>

<section aria-live="polite" class="max-h-48 shrink-0 overflow-y-auto" data-task-quick-add-outcomes>
	{#if outcomes.length === 0}
		<p class="text-sm text-muted-foreground">{text.quickAddNothing}</p>
	{:else}
		<ul class="divide-y divide-border">
			{#each outcomes as outcome, index (index)}
				<li class="flex items-start gap-2 py-2 text-sm">
					{#if outcome.kind === 'added'}
						<CheckIcon class="mt-0.5 size-4 shrink-0 text-primary" aria-label={text.quickAddAdded} />
					{:else}
						<CircleAlertIcon class="mt-0.5 size-4 shrink-0 text-destructive" aria-label={text.quickAddRefused} />
					{/if}
					<div class="min-w-0 flex-1">
						<p class="truncate font-medium">{outcome.title}</p>
						{#if outcome.kind === 'added'}
							{#if addedDetail(outcome)}<p class="text-xs text-muted-foreground">{addedDetail(outcome)}</p>{/if}
						{:else}
							<p class="break-words text-xs text-muted-foreground">{outcome.reason}</p>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</section>
