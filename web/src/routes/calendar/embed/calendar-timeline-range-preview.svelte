<script lang="ts">
	import type { TimelineRangePreviewSegment } from './calendar-timeline-preview';

	type CalendarTimelineRangePreviewProps = {
		segments: TimelineRangePreviewSegment[];
		title: string;
	};

	let { segments, title }: CalendarTimelineRangePreviewProps = $props();
</script>

{#each segments as segment (segment.id)}
	<div
		class="timeline-range-preview"
		class:calendar-timeline-overlap-adjusted={segment.isOverlapped}
		style:top={`${segment.top}px`}
		style:left={`${segment.left}px`}
		style:width={`${segment.width}px`}
		style:height={`${segment.height}px`}
		style:--calendar-overlap-left={segment.overlapLeft}
		style:--calendar-overlap-width={segment.overlapWidth}
		data-timeline-preview-id={segment.id}
	>
		<span class="timeline-range-preview-title">{title}</span>
		<span class="timeline-range-preview-time">{segment.timeLabel}</span>
	</div>
{/each}

<style>
	.timeline-range-preview {
		position: absolute;
		z-index: 45;
		display: flex;
		min-height: 24px;
		flex-direction: column;
		justify-content: flex-start;
		gap: 2px;
		overflow: hidden;
		border: 1px solid color-mix(in oklab, oklch(0.55 0.19 255) 72%, white);
		border-radius: 6px;
		background: color-mix(in oklab, oklch(0.55 0.19 255) 18%, white);
		padding: 4px 6px;
		color: #1d4ed8;
		pointer-events: none;
		box-shadow: 0 8px 18px rgb(37 99 235 / 0.16);
	}

	.timeline-range-preview-title,
	.timeline-range-preview-time {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 11px;
		line-height: 1.15;
	}

	.timeline-range-preview-title {
		font-weight: 700;
	}

	.timeline-range-preview-time {
		color: #2563eb;
		font-weight: 600;
	}

	:global(html.dark) .timeline-range-preview {
		border-color: color-mix(in oklab, oklch(0.64 0.18 255) 60%, black);
		background: color-mix(in oklab, oklch(0.64 0.18 255) 22%, #09090b);
		color: #bfdbfe;
	}

	:global(html.dark) .timeline-range-preview-time {
		color: #93c5fd;
	}
</style>
