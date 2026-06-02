<script lang="ts">
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';

	type CalendarMonthRangePreviewProps = {
		segments: MonthRangePreviewSegment[];
		title: string;
	};

	let { segments, title }: CalendarMonthRangePreviewProps = $props();
</script>

{#each segments as segment (segment.id)}
	<div
		class="month-range-preview"
		style={`left: ${segment.left}px; top: ${segment.top}px; width: ${segment.width}px; height: ${segment.height}px;`}
	>
		<span class="month-range-preview-dot"></span>
		<span class="month-range-preview-title">{title}</span>
	</div>
{/each}

<style>
	.month-range-preview {
		position: absolute;
		z-index: 15;
		display: flex;
		align-items: center;
		gap: 6px;
		overflow: hidden;
		border-radius: 4px;
		background: var(--primary);
		padding: 0 8px;
		color: var(--primary-foreground);
		font-size: 12px;
		font-weight: 600;
		line-height: 1;
		pointer-events: none;
		box-shadow:
			0 1px 2px rgb(15 23 42 / 0.18),
			inset 0 0 0 1px rgb(255 255 255 / 0.18);
	}

	.month-range-preview-dot {
		width: 6px;
		height: 6px;
		flex: 0 0 auto;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--primary-foreground) 80%, transparent);
	}

	.month-range-preview-title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
