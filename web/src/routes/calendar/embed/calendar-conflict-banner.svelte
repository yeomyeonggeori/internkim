<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { calendarText } from '../text';
	import type { CalendarConflict } from './calendar-conflicts';

	type CalendarConflictBannerProps = {
		conflicts: CalendarConflict[];
		dismissConflict: (conflictID: number) => void | Promise<void>;
		refreshConflicts: () => void | Promise<void>;
	};

	let { conflicts, dismissConflict, refreshConflicts }: CalendarConflictBannerProps = $props();
	const text = createPageText(calendarText);

	function calendarConflictFieldLabel(field: string): string {
		return text.conflictField[field] ?? field;
	}
</script>

{#if conflicts.length > 0}
	<aside class="calendar-conflict-banner" role="alert" aria-live="polite">
		<div class="conflict-banner-header">
			<p class="conflict-banner-title">{text.conflictBannerTitle}</p>
			<p class="conflict-banner-description">{text.conflictBannerDescription}</p>
		</div>
		<ul class="conflict-banner-list">
			{#each conflicts as conflict (conflict.id)}
				<li class="conflict-banner-item">
					<span class="conflict-banner-field">{calendarConflictFieldLabel(conflict.field)}</span>
					<span class="conflict-banner-values">
						<span><em>{text.conflictMine}:</em> {conflict.localValue || '—'}</span>
						<span><em>{text.conflictRemote}:</em> {conflict.remoteValue || '—'}</span>
					</span>
					<button
						type="button"
						class="conflict-banner-action"
						onclick={() => dismissConflict(conflict.id)}
					>
						{text.conflictDismiss}
					</button>
				</li>
			{/each}
		</ul>
		<button type="button" class="conflict-banner-refresh" onclick={() => refreshConflicts()}>
			{text.conflictRefresh}
		</button>
	</aside>
{/if}

<style>
	.calendar-conflict-banner {
		margin: 12px 16px 0;
		padding: 12px 16px;
		border-radius: var(--radius);
		background-color: var(--warning-subtle);
		border: 1px solid var(--warning);
		color: var(--warning-subtle-foreground);
		display: flex;
		flex-direction: column;
		gap: 8px;
		font-size: 13px;
	}

	.conflict-banner-title {
		margin: 0;
		font-weight: 600;
		font-size: 14px;
	}

	.conflict-banner-description {
		margin: 0;
		color: var(--warning-subtle-foreground);
		opacity: 0.85;
	}

	.conflict-banner-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.conflict-banner-item {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		padding: 6px 8px;
		background-color: color-mix(in oklab, hsl(var(--card)) 70%, transparent);
		border-radius: 6px;
	}

	.conflict-banner-field {
		font-weight: 600;
		min-width: 80px;
	}

	.conflict-banner-values {
		display: flex;
		flex: 1;
		flex-wrap: wrap;
		gap: 12px;
		color: var(--warning-subtle-foreground);
	}

	.conflict-banner-values em {
		font-style: normal;
		color: var(--warning-subtle-foreground);
		opacity: 0.7;
		margin-right: 4px;
	}

	.conflict-banner-action,
	.conflict-banner-refresh {
		appearance: none;
		border: 1px solid var(--warning);
		background: transparent;
		color: var(--warning-subtle-foreground);
		padding: 4px 10px;
		border-radius: 6px;
		font-size: 12px;
		cursor: pointer;
	}

	.conflict-banner-action:hover,
	.conflict-banner-refresh:hover {
		background-color: color-mix(in oklab, var(--warning) 10%, transparent);
	}

	.conflict-banner-refresh {
		align-self: flex-end;
	}
</style>
