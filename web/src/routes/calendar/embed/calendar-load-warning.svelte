<script lang="ts">
	type CalendarLoadWarningProps = {
		message: string;
	};

	let { message }: CalendarLoadWarningProps = $props();
</script>

{#if message}
	<aside class="calendar-load-warning" role="status" aria-live="polite">
		{message}
	</aside>
{/if}

<style>
	.calendar-load-warning {
		margin: 12px 16px 0;
		padding: 10px 14px;
		border: 1px solid var(--warning);
		border-radius: var(--radius);
		background-color: var(--warning-subtle);
		color: var(--warning-subtle-foreground);
		font-size: 13px;
	}
</style>
