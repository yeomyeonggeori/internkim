<!-- 캘린더 월 뷰 스크롤 중 현재 월 overlay label을 렌더링합니다. -->
<script lang="ts">
	export type MonthScrollOverlayLabel = {
		id: string;
		text: string;
		top: number;
		direction: 1 | -1;
		isFading: boolean;
	};

	type CalendarMonthScrollOverlayProps = {
		labels: MonthScrollOverlayLabel[];
	};

	let { labels }: CalendarMonthScrollOverlayProps = $props();
</script>

{#each labels as label (label.id)}
	<div
		class="month-scroll-overlay"
		class:month-scroll-overlay-fading={label.isFading}
		class:month-scroll-overlay-next={label.direction > 0}
		class:month-scroll-overlay-previous={label.direction < 0}
		style:top={`${label.top}px`}
	>
		{label.text}
	</div>
{/each}

<style>
	.month-scroll-overlay {
		position: absolute;
		left: 24px;
		z-index: 20;
		color: rgb(24 24 27 / 0.72);
		font-size: 22px;
		font-weight: 800;
		line-height: 1;
		letter-spacing: 0;
		pointer-events: none;
		text-shadow: 0 4px 16px rgb(255 255 255 / 0.72);
		opacity: 1;
	}

	.month-scroll-overlay-fading {
		opacity: 0;
		transition: opacity 180ms ease-out;
	}

	:global(html.dark) .month-scroll-overlay {
		color: rgb(244 244 245 / 0.74);
		text-shadow: 0 4px 16px rgb(0 0 0 / 0.45);
	}
</style>
