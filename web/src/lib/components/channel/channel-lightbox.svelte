<script lang="ts" module>
	export type LightboxItem = {
		kind: 'image' | 'video';
		source: string;
		filename: string;
		width?: number;
		height?: number;
	};
	export type LightboxView = { items: LightboxItem[]; index: number };
</script>

<script lang="ts">
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import XIcon from '@lucide/svelte/icons/x';
	import ZoomInIcon from '@lucide/svelte/icons/zoom-in';
	import ZoomOutIcon from '@lucide/svelte/icons/zoom-out';
	import { fade } from 'svelte/transition';
	import LoadingImage from '$lib/components/loading-image.svelte';
	import { Slider } from '$lib/components/ui/slider/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { startDownload } from './attachment-download';
	import ChannelVideoPlayer from './channel-video-player.svelte';

	let { view = $bindable() }: { view: LightboxView | null } = $props();

	const text = createPageText(channelText);
	const smallestZoomPercent = 100;
	const largestZoomPercent = 300;
	const zoomStepPercent = 10;

	let touchStartX = 0;
	let touchMoved = false;
	let zoomPercent = $state(smallestZoomPercent);

	$effect(() => {
		if (!view) zoomPercent = smallestZoomPercent;
	});

	function step(delta: number) {
		if (!view || view.items.length < 2) return;
		const count = view.items.length;
		zoomPercent = smallestZoomPercent;
		view = { items: view.items, index: (view.index + delta + count) % count };
	}

	function zoomBy(deltaPercent: number) {
		zoomPercent = Math.min(largestZoomPercent, Math.max(smallestZoomPercent, zoomPercent + deltaPercent));
	}

	function handleKeydown(event: KeyboardEvent) {
		if (!view || event.defaultPrevented) return;
		if (event.key === 'Escape') view = null;
		else if (event.key === 'ArrowRight') step(1);
		else if (event.key === 'ArrowLeft') step(-1);
	}

	function handleTouchStart(event: TouchEvent) {
		touchStartX = event.changedTouches[0]?.clientX ?? 0;
		touchMoved = false;
	}

	function handleTouchEnd(event: TouchEvent) {
		const deltaX = (event.changedTouches[0]?.clientX ?? 0) - touchStartX;
		if (Math.abs(deltaX) < 40) return;
		touchMoved = true;
		step(deltaX < 0 ? 1 : -1);
	}

	function keepTouchOnPlayer(event: TouchEvent) {
		event.stopPropagation();
	}

	function closeFromBackdrop() {
		if (touchMoved) {
			touchMoved = false;
			return;
		}
		view = null;
	}
</script>

<svelte:window onkeydown={handleKeydown} />
{#if view}
	{@const currentItem = view.items[view.index]}
	{@const hasMultiple = view.items.length > 1}
	<div
		class="fixed inset-0 z-(--layer-alert) flex items-center justify-center"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		transition:fade={{ duration: 150 }}
		ontouchstart={handleTouchStart}
		ontouchend={handleTouchEnd}
	>
		<button
			type="button"
			class="absolute inset-0 cursor-zoom-out bg-black/80"
			aria-label={text.closeViewer}
			onclick={closeFromBackdrop}
		></button>
		{#key view.index}
			{#if currentItem.kind === 'video'}
				<div class="z-10" role="presentation" ontouchstart={keepTouchOnPlayer} ontouchend={keepTouchOnPlayer}>
					<ChannelVideoPlayer
						source={currentItem.source}
						width={currentItem.width}
						height={currentItem.height}
						variant="viewer"
					/>
				</div>
			{:else}
				<div class="pointer-events-none z-10 transition-transform" style:transform={`scale(${zoomPercent / 100})`}>
					<LoadingImage
						src={currentItem.source}
						width={currentItem.width}
						height={currentItem.height}
						alt=""
						loading="eager"
						maxHeight="calc(100dvh - 3rem)"
						class="max-w-[calc(100vw-3rem)] rounded-md"
					/>
				</div>
			{/if}
		{/key}
		<div class="absolute inset-x-0 top-0 z-20 flex items-center gap-3 p-3 text-white">
			<span class="min-w-0 flex-1 truncate text-sm text-white/90">{currentItem.filename}</span>
			<button
				type="button"
				class="flex size-9 items-center justify-center rounded-full transition hover:bg-white/20"
				aria-label={text.downloadAttachment}
				onclick={() => startDownload(currentItem.source, currentItem.filename)}
			>
				<DownloadIcon class="size-5" />
			</button>
			{#if currentItem.kind === 'image'}
				<div class="flex items-center gap-2 max-md:hidden">
					<button
						type="button"
						class="flex size-9 items-center justify-center rounded-full transition hover:bg-white/20"
						aria-label={text.zoomOut}
						onclick={() => zoomBy(-zoomStepPercent)}
					>
						<ZoomOutIcon class="size-5" />
					</button>
					<Slider
						type="single"
						bind:value={zoomPercent}
						min={smallestZoomPercent}
						max={largestZoomPercent}
						step={zoomStepPercent}
						class="w-28"
					/>
					<button
						type="button"
						class="flex size-9 items-center justify-center rounded-full transition hover:bg-white/20"
						aria-label={text.zoomIn}
						onclick={() => zoomBy(zoomStepPercent)}
					>
						<ZoomInIcon class="size-5" />
					</button>
					<span class="w-12 text-sm tabular-nums text-white/90">{zoomPercent}%</span>
				</div>
			{/if}
			<button
				type="button"
				class="flex size-9 items-center justify-center rounded-full transition hover:bg-white/20"
				aria-label={text.closeViewer}
				onclick={() => (view = null)}
			>
				<XIcon class="size-5" />
			</button>
		</div>
		{#if hasMultiple}
			<button
				type="button"
				class="absolute left-3 z-20 flex size-11 items-center justify-center rounded-full bg-white/10 text-white backdrop-blur transition hover:bg-white/20 max-md:hidden"
				aria-label="이전 이미지"
				onclick={() => step(-1)}
			>
				<ChevronLeftIcon class="size-6" />
			</button>
			<button
				type="button"
				class="absolute right-3 z-20 flex size-11 items-center justify-center rounded-full bg-white/10 text-white backdrop-blur transition hover:bg-white/20 max-md:hidden"
				aria-label="다음 이미지"
				onclick={() => step(1)}
			>
				<ChevronRightIcon class="size-6" />
			</button>
			<div
				class="absolute bottom-5 z-20 rounded-full bg-black/50 px-3 py-1 text-sm text-white/90"
			>
				{view.index + 1} / {view.items.length}
			</div>
		{/if}
	</div>
{/if}
