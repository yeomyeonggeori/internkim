<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
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
			{#each rows as row (`${row.label}:${row.actor.email}:${row.actor.name}`)}
				<div class="event-audit-row">
					<span class="event-audit-label">{row.label}</span>
					<span class="event-audit-person" title={row.actor.email || row.actor.name}>
						<PersonAvatar name={row.actor.name} email={row.actor.email} image={row.actor.image} class="size-6" />
						<strong>{row.actor.name}</strong>
					</span>
					{#if row.time}
						<time>{row.time}</time>
					{/if}
				</div>
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

	.event-audit-row {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 8px;
		margin: 0;
	}

	.event-audit-label {
		color: #6b7280;
		font-weight: 600;
	}

	.event-audit-person {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 8px;
	}

	.event-audit-person strong {
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

	:global(html.dark) .event-audit-label,
	:global(html.dark) .event-audit-empty,
	:global(html.dark) .event-audit-card time {
		color: #a1a1aa;
	}
</style>
