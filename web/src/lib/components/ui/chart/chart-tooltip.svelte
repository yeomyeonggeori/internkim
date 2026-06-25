<script lang="ts">
	import { cn, type WithElementRef, type WithoutChildren } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";
	import { getPayloadConfigFromPayload, useChart, type TooltipPayload } from "./chart-utils.js";
	import { getChartContext, Tooltip as TooltipPrimitive } from "layerchart";
	import type { Snippet } from "svelte";

	type TooltipAnchor =
		| "top-left"
		| "top"
		| "top-right"
		| "left"
		| "center"
		| "right"
		| "bottom-left"
		| "bottom"
		| "bottom-right";
	type TooltipMotion = "spring" | "tween" | "none" | { type: "spring" } | { type: "tween" } | { type: "none" };

	function defaultFormatter(value: unknown, _payload: TooltipPayload[]) {
		return `${value}`;
	}

	let {
		ref = $bindable(null),
		class: className,
		hideLabel = false,
		indicator = "dot",
		hideIndicator = false,
		labelKey,
		label,
		labelFormatter = defaultFormatter,
		labelClassName,
		formatter,
		nameKey,
		color,
		clampToContainer = false,
		anchor,
		contained,
		x,
		y,
		xOffset,
		yOffset,
		motion,
		style: styleValue,
		...restProps
	}: WithoutChildren<WithElementRef<HTMLAttributes<HTMLDivElement>>> & {
		hideLabel?: boolean;
		label?: string;
		indicator?: "line" | "dot" | "dashed";
		nameKey?: string;
		labelKey?: string;
		hideIndicator?: boolean;
		labelClassName?: string;
		labelFormatter?: ((value: unknown, payload: TooltipPayload[]) => string | number | Snippet) | null;
		clampToContainer?: boolean;
		anchor?: TooltipAnchor;
		contained?: "container" | "window" | false;
		x?: "pointer" | "data" | number;
		y?: "pointer" | "data" | number;
		xOffset?: number;
		yOffset?: number;
		motion?: TooltipMotion;
		formatter?: Snippet<
			[
				{
					value: unknown;
					name: string;
					item: TooltipPayload;
					index: number;
					payload: TooltipPayload[];
				},
			]
		>;
	} = $props();

	const chart = useChart();
	const chartCtx = getChartContext();
	let tooltipContent = $state<HTMLDivElement | null>(null);
	let clampOffset = $state({ x: 0, y: 0 });
	const shouldClampToContainer = $derived(clampToContainer || contained === false);

	const tooltipStyle = $derived(
		[
			styleValue,
			shouldClampToContainer
				? `transform: translate(${clampOffset.x}px, ${clampOffset.y}px);`
				: "",
		]
			.filter(Boolean)
			.join(" ")
	);

	const visibleSeries = $derived(
		chartCtx.tooltip.series.filter((s: TooltipPayload) => s.value !== undefined)
	);

	const formattedLabel = $derived.by(() => {
		if (hideLabel || !visibleSeries?.length) return null;

		const [item] = visibleSeries;
		const tooltipData = chartCtx.tooltip.data;

		const dataLabel = tooltipData != null ? chartCtx.x(tooltipData) : undefined;

		const key = labelKey ?? item?.label ?? item?.key ?? "value";
		const itemConfig = getPayloadConfigFromPayload(
			chart.config,
			item,
			key,
			tooltipData as Record<string, unknown> | null
		);

		let value: unknown;
		if (!labelKey && typeof label === "string") {
			value = chart.config[label as keyof typeof chart.config]?.label ?? label;
		} else if (labelKey) {
			value = itemConfig?.label ?? dataLabel;
		} else {
			value = dataLabel;
		}

		if (value === undefined) return null;
		if (!labelFormatter) return value;
		return labelFormatter(value, visibleSeries);
	});

	const nestLabel = $derived(visibleSeries.length === 1 && indicator !== "dot");

	$effect(() => {
		ref = tooltipContent;
	});

	$effect(() => {
		if (!shouldClampToContainer || !chartCtx.tooltip.data || !tooltipContent) {
			clampOffset = { x: 0, y: 0 };
			return;
		}
		chartCtx.tooltip.x;
		chartCtx.tooltip.y;
		requestAnimationFrame(updateClampOffset);
	});

	function updateClampOffset() {
		if (!tooltipContent) return;
		const container = tooltipContent.closest("[data-chart]") ?? chartCtx.containerRef;
		if (!container) return;

		const margin = 2;
		const tooltipRectangle = tooltipContent.getBoundingClientRect();
		const containerRectangle = container.getBoundingClientRect();
		const baseRectangle = {
			left: tooltipRectangle.left - clampOffset.x,
			right: tooltipRectangle.right - clampOffset.x,
			top: tooltipRectangle.top - clampOffset.y,
			bottom: tooltipRectangle.bottom - clampOffset.y,
			width: tooltipRectangle.width,
			height: tooltipRectangle.height,
		};
		const nextOffset = calculateClampOffset(baseRectangle, containerRectangle, margin);

		if (clampOffset.x !== nextOffset.x || clampOffset.y !== nextOffset.y) {
			clampOffset = nextOffset;
		}
	}

	function calculateClampOffset(
		tooltipRectangle: Pick<DOMRect, "left" | "right" | "top" | "bottom" | "width" | "height">,
		containerRectangle: Pick<DOMRect, "left" | "right" | "top" | "bottom" | "width" | "height">,
		margin: number
	) {
		const minimumLeft = containerRectangle.left + margin;
		const maximumRight = containerRectangle.right - margin;
		const minimumTop = containerRectangle.top + margin;
		const maximumBottom = containerRectangle.bottom - margin;
		let x = 0;
		let y = 0;

		if (tooltipRectangle.width <= maximumRight - minimumLeft) {
			if (tooltipRectangle.left < minimumLeft) x = minimumLeft - tooltipRectangle.left;
			if (tooltipRectangle.right + x > maximumRight) x += maximumRight - (tooltipRectangle.right + x);
		} else {
			x = minimumLeft - tooltipRectangle.left;
		}

		if (tooltipRectangle.height <= maximumBottom - minimumTop) {
			if (tooltipRectangle.top < minimumTop) y = minimumTop - tooltipRectangle.top;
			if (tooltipRectangle.bottom + y > maximumBottom) y += maximumBottom - (tooltipRectangle.bottom + y);
		} else {
			y = minimumTop - tooltipRectangle.top;
		}

		return { x, y };
	}
</script>

{#snippet TooltipLabel()}
	{#if formattedLabel}
		<div class={cn("font-medium", labelClassName)}>
			{#if typeof formattedLabel === "function"}
				{@render formattedLabel()}
			{:else}
				{formattedLabel}
			{/if}
		</div>
	{/if}
{/snippet}

<TooltipPrimitive.Root
	{anchor}
	{contained}
	{x}
	{y}
	{xOffset}
	{yOffset}
	{motion}
	variant="none"
>
	<div
		bind:this={tooltipContent}
		class={cn(
			"border-border/50 bg-background grid min-w-[9rem] items-start gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs shadow-xl",
			className
		)}
		style={tooltipStyle}
		{...restProps}
	>
		{#if !nestLabel}
			{@render TooltipLabel()}
		{/if}
		<div class="grid gap-1.5">
			{#each visibleSeries as item, i (item.key + i)}
				{@const key = `${nameKey || item.key || item.label || "value"}`}
				{@const itemConfig = getPayloadConfigFromPayload(
					chart.config,
					item,
					key,
					chartCtx.tooltip.data
				)}
				{@const indicatorColor = color || item.config?.color || item.color}
				<div
					class={cn(
						"[&>svg]:text-muted-foreground flex w-full flex-wrap items-stretch gap-2 [&>svg]:size-2.5",
						indicator === "dot" && "items-center"
					)}
				>
					{#if formatter && item.value !== undefined && item.label}
						{@render formatter({
							value: item.value,
							name: item.label,
							item,
							index: i,
							payload: visibleSeries,
						})}
					{:else}
						{#if itemConfig?.icon}
							<itemConfig.icon />
						{:else if !hideIndicator}
							<div
								style="--color-bg: {indicatorColor}; --color-border: {indicatorColor};"
								class={cn(
									"shrink-0 rounded-[2px] border-(--color-border) bg-(--color-bg)",
									{
										"size-2.5": indicator === "dot",
										"h-full w-1": indicator === "line",
										"w-0 border-[1.5px] border-dashed bg-transparent":
											indicator === "dashed",
										"my-0.5": nestLabel && indicator === "dashed",
									}
								)}
							></div>
						{/if}
						<div
							class={cn(
								"flex flex-1 shrink-0 justify-between leading-none",
								nestLabel ? "items-end" : "items-center"
							)}
						>
							<div class="grid gap-1.5">
								{#if nestLabel}
									{@render TooltipLabel()}
								{/if}
								<span class="text-muted-foreground">
									{itemConfig?.label || item.label}
								</span>
							</div>
							{#if item.value !== undefined}
								<span class="text-foreground font-mono font-medium tabular-nums">
									{item.value.toLocaleString()}
								</span>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>
</TooltipPrimitive.Root>
