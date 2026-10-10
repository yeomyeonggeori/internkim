<script lang="ts">
	import type { Attachment } from 'svelte/attachments';
	import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from 'pdfjs-dist';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import ScanIcon from '@lucide/svelte/icons/scan';
	import { Spinner } from '$lib/components/ui/spinner';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { previewText } from './text';

	let { bytes, onFailure }: { bytes: ArrayBuffer; onFailure: (error: unknown) => void } = $props();

	const text = createPageText(previewText);
	const zoomSteps = [0.5, 0.75, 1, 1.25, 1.5, 2];
	const maxFittedWidth = 960;
	const pagePadding = 32;
	let pdfDocument = $state.raw<PDFDocumentProxy | null>(null);
	let firstPageRatio = $state(Math.SQRT2);
	let zoomIndex = $state(2);
	let containerWidth = $state(0);
	const pageWidth = $derived(
		Math.max(240, Math.min(containerWidth - pagePadding, maxFittedWidth)) * zoomSteps[zoomIndex]
	);
	const pageNumbers = $derived(
		Array.from({ length: pdfDocument?.numPages ?? 0 }, (_, index) => index + 1)
	);

	$effect(() => {
		const opening: { task: PDFDocumentLoadingTask | null; isClosed: boolean } = { task: null, isClosed: false };
		void openDocument(bytes, opening);
		return () => {
			opening.isClosed = true;
			void opening.task?.destroy();
		};
	});

	async function openDocument(source: ArrayBuffer, opening: { task: PDFDocumentLoadingTask | null; isClosed: boolean }) {
		try {
			const pdfjs = await import('pdfjs-dist');
			const { default: workerURL } = await import('pdfjs-dist/build/pdf.worker.min.mjs?url');
			if (opening.isClosed) return;
			pdfjs.GlobalWorkerOptions.workerSrc = workerURL;
			opening.task = pdfjs.getDocument({ data: new Uint8Array(source.slice(0)) });
			const opened = await opening.task.promise;
			const firstPage = (await opened.getPage(1)).getViewport({ scale: 1 });
			firstPageRatio = firstPage.height / firstPage.width;
			pdfDocument = opened;
		} catch (error) {
			if (!opening.isClosed) onFailure(error);
		}
	}

	function renderedPage(opened: PDFDocumentProxy, pageNumber: number, width: number): Attachment<HTMLCanvasElement> {
		return (canvas) => {
			let renderTask: RenderTask | null = null;
			let isDetached = false;
			const observer = new IntersectionObserver(
				([entry]) => {
					if (!entry?.isIntersecting) return;
					observer.disconnect();
					void draw();
				},
				{ rootMargin: '800px 0px' }
			);
			async function draw() {
				const page = await opened.getPage(pageNumber);
				if (isDetached) return;
				const scale = (width / page.getViewport({ scale: 1 }).width) * window.devicePixelRatio;
				const viewport = page.getViewport({ scale });
				canvas.width = Math.floor(viewport.width);
				canvas.height = Math.floor(viewport.height);
				canvas.style.aspectRatio = `${viewport.width} / ${viewport.height}`;
				renderTask = page.render({ canvas, viewport });
				await renderTask.promise.catch((error: unknown) => {
					if (!(error instanceof Error) || error.name !== 'RenderingCancelledException') onFailure(error);
				});
			}
			observer.observe(canvas);
			return () => {
				isDetached = true;
				observer.disconnect();
				renderTask?.cancel();
			};
		};
	}
</script>

<div class="flex h-full min-h-0 flex-col">
	<div class="bg-background/80 flex items-center justify-between gap-2 border-b px-3 py-1.5 text-xs backdrop-blur">
		<span class="text-muted-foreground tabular-nums">
			{#if pdfDocument}{pdfDocument.numPages} {text.pages}{/if}
		</span>
		<div class="flex items-center gap-1">
			<TooltipIconButton
				label={text.zoomOut}
				variant="ghost"
				size="icon-sm"
				disabled={zoomIndex === 0}
				onclick={() => (zoomIndex -= 1)}><MinusIcon /></TooltipIconButton
			>
			<span class="w-10 text-center tabular-nums">{Math.round(zoomSteps[zoomIndex] * 100)}%</span>
			<TooltipIconButton
				label={text.zoomIn}
				variant="ghost"
				size="icon-sm"
				disabled={zoomIndex === zoomSteps.length - 1}
				onclick={() => (zoomIndex += 1)}><PlusIcon /></TooltipIconButton
			>
			<TooltipIconButton label={text.fitWidth} variant="ghost" size="icon-sm" onclick={() => (zoomIndex = 2)}
				><ScanIcon /></TooltipIconButton
			>
		</div>
	</div>
	<div class="min-h-0 flex-1 overflow-auto" bind:clientWidth={containerWidth}>
		{#if pdfDocument}
			<div class="flex w-max min-w-full flex-col items-center gap-4 p-4">
				{#each pageNumbers as pageNumber (pageNumber)}
					{#key pageWidth}
						<canvas
							aria-label="{pageNumber} / {pdfDocument.numPages}"
							class="bg-white shadow-sm ring-1 ring-black/5"
							style:width="{pageWidth}px"
							style:aspect-ratio="1 / {firstPageRatio}"
							{@attach renderedPage(pdfDocument, pageNumber, pageWidth)}
						></canvas>
					{/key}
				{/each}
			</div>
		{:else}
			<div class="flex h-full items-center justify-center"><Spinner /></div>
		{/if}
	</div>
</div>
