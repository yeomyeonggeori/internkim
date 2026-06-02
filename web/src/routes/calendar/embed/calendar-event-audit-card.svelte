<script lang="ts">
	import type { CalendarAuditRow } from './calendar-audit';

	type CalendarEventAuditCardProps = {
		rows: CalendarAuditRow[];
		label: string;
	};

	let { rows, label }: CalendarEventAuditCardProps = $props();
</script>

{#if rows.length > 0}
	<aside class="event-audit-card" aria-label={label}>
		{#each rows as row (row.label)}
			<p>
				<span>{row.label}</span>
				<strong>{row.person}</strong>
				{#if row.time}
					<time>{row.time}</time>
				{/if}
			</p>
		{/each}
	</aside>
{/if}

<style>
	.event-audit-card {
		position: absolute;
		right: 16px;
		bottom: 16px;
		z-index: 20;
		display: grid;
		gap: 6px;
		max-width: min(360px, calc(100vw - 32px));
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.96);
		padding: 10px 12px;
		box-shadow: 0 12px 30px rgba(15, 23, 42, 0.14);
		color: #111827;
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

	:global(html.dark) .event-audit-card {
		border-color: #27272a;
		background: rgba(9, 9, 11, 0.96);
		color: #f4f4f5;
		box-shadow: 0 12px 30px rgba(0, 0, 0, 0.36);
	}

	:global(html.dark) .event-audit-card span,
	:global(html.dark) .event-audit-card time {
		color: #a1a1aa;
	}
</style>
