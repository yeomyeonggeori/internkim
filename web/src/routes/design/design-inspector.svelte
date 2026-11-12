<script lang="ts">
	import { tick } from 'svelte';
	import { CopyButton } from '$lib/components/ui/copy-button/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { tokensUsedIn, type FrameTokens } from './frame-tokens';
	import { designText } from './text';
	import { useWorkspace } from './workspace.svelte';

	let { pageName, pageDescription }: { pageName: string; pageDescription: string } = $props();

	const text = createPageText(designText);
	const workspace = useWorkspace();
	const frame = $derived(workspace.selectedFrame);
	const sourcePath = $derived(frame?.source ? `web/src/lib/components/${frame.source}` : '');
	const importLine = $derived(frame?.source.startsWith('ui/') ? importLineFor(frame.source) : '');

	function importLineFor(source: string): string {
		const directory = source.slice('ui/'.length);
		const moduleName = directory
			.split('-')
			.map((word) => word.charAt(0).toUpperCase() + word.slice(1))
			.join('');
		return `import * as ${moduleName} from '$lib/components/${source}';`;
	}

	let tokens = $state<FrameTokens>({ colors: [], radii: [] });
	let size = $state('');

	$effect(() => {
		const id = workspace.selectedID;
		void workspace.themes;
		if (!id) return;
		let observer: ResizeObserver | undefined;
		tick().then(() => {
			const pane = document.querySelector(`[data-frame="${CSS.escape(id)}"] [data-theme-pane]`);
			if (!pane) return;
			tokens = tokensUsedIn(pane);
			observer = new ResizeObserver(() => {
				const box = pane.getBoundingClientRect();
				size = `${Math.round(box.width / (workspace.zoom / 100))} × ${Math.round(box.height / (workspace.zoom / 100))}`;
			});
			observer.observe(pane);
		});
		return () => observer?.disconnect();
	});
</script>

{#snippet property(label: string)}
	<dt class="text-muted-foreground text-[11px] font-medium">{label}</dt>
{/snippet}

<div class="grid content-start gap-5 p-4 text-sm">
	{#if frame}
		<div class="grid gap-1">
			<span class="text-muted-foreground text-[11px] font-medium">{text.frameLabel}</span>
			<h2 class="text-base leading-tight font-semibold break-words">{frame.name}</h2>
			<span class="text-muted-foreground text-xs">{frame.sectionName}</span>
		</div>
		{#if frame.note}<p class="text-muted-foreground leading-relaxed">{frame.note}</p>{/if}
		<dl class="grid gap-x-3 gap-y-2.5">
			{#if size}
				{@render property(text.sizeLabel)}
				<dd class="font-mono text-xs tabular-nums">{size}</dd>
			{/if}
			{#if sourcePath}
				{@render property(text.sourceLabel)}
				<dd class="flex min-w-0 items-center gap-1">
					<code class="bg-muted min-w-0 truncate rounded-sm px-1.5 py-0.5 font-mono text-[11px]" title={sourcePath}>{sourcePath}</code>
					{#if importLine}<CopyButton text={importLine} size="icon-xs" />{/if}
				</dd>
			{/if}
		</dl>
		{#if frame.variants.length > 0}
			<section class="grid gap-2">
				<h3 class="text-muted-foreground text-[11px] font-medium">{text.variantsLabel}</h3>
				{#each frame.variants as axis (axis.name)}
					<div class="grid gap-1.5">
						<span class="font-mono text-xs">{axis.name}</span>
						<div class="flex flex-wrap gap-1">
							{#each axis.values as value (value)}
								<span class="bg-muted rounded-sm px-1.5 py-0.5 font-mono text-[11px]">{value}</span>
							{/each}
						</div>
					</div>
				{/each}
			</section>
		{/if}
		{#if tokens.colors.length > 0 || tokens.radii.length > 0}
			<section class="grid gap-2">
				<h3 class="text-muted-foreground text-[11px] font-medium">{text.tokensLabel}</h3>
				<ul class="grid gap-1.5">
					{#each tokens.colors as name (name)}
						<li class="flex items-center gap-2">
							<span class="flex overflow-hidden rounded-sm ring-1 ring-foreground/15">
								<span class="light size-4" style:background={`var(--${name})`}></span>
								<span class="dark size-4" style:background={`var(--${name})`}></span>
							</span>
							<span class="font-mono text-xs">{name}</span>
						</li>
					{/each}
					{#each tokens.radii as name (name)}
						<li class="flex items-center gap-2">
							<span class="border-foreground/40 size-4 border-t-2 border-l-2" style:border-top-left-radius={`var(--radius-${name})`}></span>
							<span class="font-mono text-xs">radius-{name}</span>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	{:else}
		<div class="grid gap-1">
			<span class="text-muted-foreground text-[11px] font-medium">{text.pageLabel}</span>
			<h2 class="text-base font-semibold">{pageName}</h2>
		</div>
		<p class="text-muted-foreground leading-relaxed">{pageDescription}</p>
		<p class="text-muted-foreground text-xs">{text.selectHint}</p>
	{/if}
</div>
