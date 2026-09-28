<script lang="ts" module>
	export type LightboxView = { images: string[]; index: number };
</script>

<script lang="ts">
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { fade, scale } from 'svelte/transition';

	let { view = $bindable() }: { view: LightboxView | null } = $props();

	let touchStartX = 0;
	let touchMoved = false;

	function step(delta: number) {
		if (!view || view.images.length < 2) return;
		const count = view.images.length;
		view = { images: view.images, index: (view.index + delta + count) % count };
	}

	function handleKeydown(event: KeyboardEvent) {
		if (!view) return;
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
	{@const currentImage = view.images[view.index]}
	{@const hasMultiple = view.images.length > 1}
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
			aria-label="이미지 닫기"
			onclick={closeFromBackdrop}
		></button>
		{#key view.index}
			<img
				src={currentImage}
				alt=""
				class="pointer-events-none relative z-10 max-h-full max-w-full rounded-md object-contain p-6"
				in:scale={{ duration: 200, start: 0.94 }}
			/>
		{/key}
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
				{view.index + 1} / {view.images.length}
			</div>
		{/if}
	</div>
{/if}
