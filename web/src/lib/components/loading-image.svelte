<script lang="ts">
	import ImageOffIcon from '@lucide/svelte/icons/image-off';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import { observeImageLoad, type ImageLoadState } from './image-loading';

	let {
		src,
		alt,
		width,
		height,
		fill = false,
		maxHeight = 'min(60vh, 26rem)',
		loading = 'lazy',
		class: className,
		imageClass,
		fallbackText,
		messagePicture = false
	}: {
		src: string;
		alt: string;
		width?: number;
		height?: number;
		fill?: boolean;
		maxHeight?: string;
		loading?: 'lazy' | 'eager';
		class?: string;
		imageClass?: string;
		fallbackText?: string;
		messagePicture?: boolean;
	} = $props();

	const text = createPageText({
		ko: { loading: '이미지를 불러오는 중', failed: '이미지를 불러올 수 없어요' },
		en: { loading: 'Loading image', failed: 'Image could not be loaded' }
	});
	let result = $state<{ source: string; state: ImageLoadState; width: number; height: number } | null>(null);
	const status = $derived(result?.source === src ? result.state : 'loading');
	const hasTextFallback = $derived(status === 'error' && Boolean(fallbackText));
	const dimensions = $derived(
		width && Number.isFinite(width) && width > 0 && height && Number.isFinite(height) && height > 0
			? { width, height }
			: result?.source === src && result.state === 'loaded' && result.width > 0 && result.height > 0
				? result
				: { width: 320, height: 320 }
	);

	function watchImage(image: HTMLImageElement) {
		const source = src;
		return observeImageLoad(image, (nextState) => {
			if (source !== src) return;
			result = { source, state: nextState, width: image.naturalWidth, height: image.naturalHeight };
		});
	}
</script>

<span
	data-loading-image={status}
	aria-busy={status === 'loading'}
	class={cn('relative block max-w-full overflow-hidden rounded-[inherit]', className)}
	style:width={hasTextFallback ? 'auto' : fill ? undefined : `min(${dimensions.width}px, calc(${maxHeight} * ${dimensions.width / dimensions.height}))`}
	style:height={hasTextFallback ? 'auto' : undefined}
	style:aspect-ratio={fill || hasTextFallback ? undefined : `${dimensions.width} / ${dimensions.height}`}
>
	{#key src}
		<img
			{src}
			{alt}
			{width}
			{height}
			{loading}
			decoding="async"
			data-message-picture={messagePicture ? '' : undefined}
			aria-hidden={status !== 'loaded' ? true : undefined}
			class={cn('absolute inset-0 size-full object-contain', status !== 'loaded' && 'invisible', imageClass)}
			use:watchImage
		/>
	{/key}
	{#if status === 'loading'}
		<span role="status" aria-label={text.loading} class="absolute inset-0">
			<Skeleton aria-hidden="true" class="size-full rounded-[inherit]" />
		</span>
	{:else if status === 'error'}
		{#if fallbackText}
			<span role="img" aria-label={`${fallbackText}: ${text.failed}`} class="block max-w-32 truncate text-xs">{fallbackText}</span>
		{:else}
			<span role="img" aria-label={alt ? `${alt}: ${text.failed}` : text.failed} class="bg-muted text-muted-foreground absolute inset-0 flex flex-col items-center justify-center gap-2 p-2 text-center text-xs">
				<ImageOffIcon aria-hidden="true" class="size-5 shrink-0" />
				<span aria-hidden="true">{text.failed}</span>
			</span>
		{/if}
	{/if}
</span>
