<script lang="ts">
	import type { CalendarAuditRow } from './calendar-audit';

type CalendarEventAuditCardProps = {
	rows: CalendarAuditRow[];
	label: string;
	emptyText?: string;
};

let { rows, label, emptyText = '' }: CalendarEventAuditCardProps = $props();
</script>

{#if rows.length > 0 || emptyText}
	<aside class="event-audit-card" aria-label={label}>
		{#if rows.length > 0}
			{#each rows as row (row.label)}
				<p>
					<span>{row.label}</span>
					<strong>{row.person}</strong>
					{#if row.time}
						<time>{row.time}</time>
					{/if}
				</p>
			{/each}
		{:else}
			<p class="event-audit-empty">{emptyText}</p>
		{/if}
	</aside>
{/if}

<style>
	.event-audit-card {
		display: grid;
		gap: 6px;
		margin-top: 10px;
		border: 1px solid rgba(148, 163, 184, 0.22);
		border-radius: 14px;
		background: rgba(241, 245, 249, 0.58);
		padding: 11px 12px;
		color: #1f2937;
		font-size: 12px;
		line-height: 1.35;
	}

	.event-audit-card p {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 8px;
		margin: 0;
	}

	.event-audit-card span {
		color: #6b7280;
		font-weight: 600;
	}

	.event-audit-card strong {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 700;
	}

	.event-audit-card time {
		color: #6b7280;
		white-space: nowrap;
	}

	.event-audit-empty {
		display: block;
		color: #6b7280;
		font-weight: 600;
	}

	:global(html.dark) .event-audit-card {
		border-color: #27272a;
		background: rgba(39, 39, 42, 0.58);
		color: #f4f4f5;
	}

	:global(html.dark) .event-audit-card span,
	:global(html.dark) .event-audit-empty,
	:global(html.dark) .event-audit-card time {
		color: #a1a1aa;
	}
</style>
