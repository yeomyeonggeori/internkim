<script lang="ts">
	import { browser } from '$app/environment';
	import { cn } from '$lib/utils';
	import { contrastGrade, contrastRatio, hexOf } from './color-contrast';
	import { colorValue, type ColorToken } from './theme-tokens';
	import type { ThemeName } from './workspace.svelte';

	let { token, theme }: { token: ColorToken; theme: ThemeName } = $props();

	const value = $derived(colorValue(token.name, theme));
	const foregroundValue = $derived(token.foreground ? colorValue(token.foreground, theme) : '');
	const hex = $derived(browser ? hexOf(value) : '');
	const ratio = $derived(browser && foregroundValue ? contrastRatio(foregroundValue, value) : undefined);
	const grade = $derived(ratio === undefined ? undefined : contrastGrade(ratio));
</script>

<div class="grid min-w-0 content-start gap-1.5" data-token={token.name}>
	<div
		class="ring-foreground/10 flex h-16 items-end justify-between rounded-md p-2 ring-1 ring-inset"
		style:background={`var(--${token.name})`}
	>
		{#if token.foreground}
			<span class="text-base leading-none font-semibold" style:color={`var(--${token.foreground})`}>Aa 가</span>
		{/if}
	</div>
	<div class="grid gap-0.5">
		<span class="truncate text-xs font-medium">{token.name}</span>
		<span class="text-muted-foreground truncate font-mono text-[10px]" title={value}>{hex || value}</span>
		{#if token.foreground}
			<span class="text-muted-foreground truncate font-mono text-[10px]">on {token.foreground}</span>
		{/if}
		{#if ratio !== undefined && grade}
			<span
				class={cn(
					'w-fit rounded-sm px-1 font-mono text-[10px] tabular-nums',
					grade === 'fail' ? 'bg-destructive/10 text-destructive' : 'bg-muted text-muted-foreground'
				)}
			>
				{ratio.toFixed(2)} · {grade}
			</span>
		{/if}
	</div>
</div>
