<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import MaximizeIcon from '@lucide/svelte/icons/maximize';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import type { UserRecord } from '../admin/admin-types';
	import type { OrgchartCanvasModel } from './orgchart-directory-model';
	import {
		orgchartBoardMinimumWidth,
		orgchartColumnLeftPositions,
		orgchartDistributedColumnSpace,
		orgchartEdgePath,
		orgchartFitZoom,
		orgchartMaximumZoom,
		orgchartMinimumZoom,
		orgchartPersonNodeWidth,
		orgchartPersonNodeHeight,
		orgchartRootConnectorEdges,
		orgchartRootLayouts,
		orgchartRootNodeWidth,
		orgchartRootRowMinimumWidth,
		orgchartOrderedTeamLayouts,
		orgchartTeamLayout
	} from './orgchart-layout';
	import OrgchartPersonNode from './orgchart-person-node.svelte';
	import type { orgchartDirectoryText } from './text';

	const orgchartConnectorLayerHeight = 80;
	const orgchartCanvasVerticalPadding = 44;

	type OrgchartOrgCanvasProps = {
		model: OrgchartCanvasModel;
		selectedUserID: string;
		zoom: number;
		text: typeof orgchartDirectoryText.ko;
		selectRecord: (record: UserRecord, anchor: DOMRect) => void;
		setZoom: (zoom: number) => void;
	};

	let { model, selectedUserID, zoom, text, selectRecord, setZoom }: OrgchartOrgCanvasProps = $props();
	let scrollElement = $state<HTMLDivElement>();
	let viewportWidth = $state(0);
	let viewportHeight = $state(0);
	let centeredBoardKey = $state('');
	let isAutoFitEnabled = $state(true);

	const zoomLabel = $derived(`${zoom}%`);
	const canvasScale = $derived(zoom / 100);
	const teamLayouts = $derived(orgchartOrderedTeamLayouts(model.roots, model.columns.map((column) => orgchartTeamLayout(column))));
	const columnWidths = $derived(teamLayouts.map((layout) => layout.width));
	const boardMinimumWidth = $derived(orgchartBoardMinimumWidth(columnWidths));
	const rootRowMinimumWidth = $derived(orgchartRootRowMinimumWidth(model.roots.length));
	const boardWidth = $derived(Math.max(boardMinimumWidth, rootRowMinimumWidth));
	const columnSpace = $derived(orgchartDistributedColumnSpace(columnWidths, boardWidth));
	const connectorWidth = $derived(boardWidth);
	const columnLefts = $derived(orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth));
	const rootLayouts = $derived(orgchartRootLayouts(model.roots, teamLayouts, columnLefts, boardWidth));
	const rootConnectorEdges = $derived(orgchartRootConnectorEdges(model.roots, teamLayouts, columnLefts, boardWidth));
	const rootNodeStyle = $derived(`width: ${orgchartRootNodeWidth}px;`);
	const boardHeight = $derived(orgchartPersonNodeHeight + orgchartConnectorLayerHeight + Math.max(0, ...teamLayouts.map((layout) => layout.height)));
	const availableViewportHeight = $derived(Math.max(viewportHeight - orgchartCanvasVerticalPadding, 0));
	const fitZoom = $derived(orgchartFitZoom({ boardWidth, boardHeight, viewportWidth, viewportHeight: availableViewportHeight }));
	const teamGridStyle = $derived(`grid-template-columns: ${columnWidths.map((width) => `${width}px`).join(' ')}; gap: ${columnSpace}px; width: ${boardWidth}px; justify-content: ${columnWidths.length === 1 ? 'center' : 'start'};`);
	const boardStyle = $derived(`min-width: ${boardWidth}px; width: ${boardWidth}px; zoom: ${canvasScale};`);

	function teamTone(): string {
		return 'border-slate-200 bg-slate-50 text-slate-800 dark:border-slate-800 dark:bg-slate-950/30 dark:text-slate-200';
	}

	function setManualZoom(nextZoom: number): void {
		isAutoFitEnabled = false;
		setZoom(nextZoom);
	}

	function setFitZoom(): void {
		isAutoFitEnabled = true;
		setZoom(fitZoom);
	}

	$effect(() => {
		const nextZoom = fitZoom;
		if (!isAutoFitEnabled || model.columns.length === 0 || zoom === nextZoom) return;
		setZoom(nextZoom);
	});

	$effect(() => {
		const nextBoardWidth = boardWidth;
		const nextBoardKey = `${nextBoardWidth}:${zoom}`;
		if (!scrollElement || model.columns.length === 0 || centeredBoardKey === nextBoardKey) return;
		requestAnimationFrame(() => {
			if (!scrollElement) return;
			scrollElement.scrollLeft = Math.max((scrollElement.scrollWidth - scrollElement.clientWidth) / 2, 0);
			centeredBoardKey = nextBoardKey;
		});
	});
</script>

<section class="flex h-full min-h-[calc(100vh-12rem)] min-w-0 flex-col" data-testid="orgchart-canvas">
	<div class="relative z-10 mb-4 flex w-fit items-center gap-1 rounded-md border bg-background p-1 shadow-sm">
		<Button variant="ghost" size="icon" aria-label={text.zoomOut} onclick={() => setManualZoom(Math.max(orgchartMinimumZoom, zoom - 10))}>
			<MinusIcon class="size-4" />
		</Button>
		<div class="min-w-16 px-3 text-center text-sm font-medium">{zoomLabel}</div>
		<Button variant="ghost" size="icon" aria-label={text.zoomIn} onclick={() => setManualZoom(Math.min(orgchartMaximumZoom, zoom + 10))}>
			<PlusIcon class="size-4" />
		</Button>
		<Button variant="ghost" size="icon" aria-label={text.zoomFit} onclick={setFitZoom}>
			<MaximizeIcon class="size-4" />
		</Button>
	</div>

	<div bind:this={scrollElement} bind:clientWidth={viewportWidth} bind:clientHeight={viewportHeight} class="min-h-0 flex-1 overflow-auto pb-8" data-testid="orgchart-canvas-scroll">
		<div class="grid min-h-full w-full content-start justify-items-center pt-3">
			<div class="grid min-h-full content-start justify-items-stretch" style={boardStyle}>
				<div class="relative h-24">
					{#each rootLayouts as root (root.record.userID)}
						<div class="absolute top-0" style={`left: ${root.x}px; ${rootNodeStyle}`}>
							<OrgchartPersonNode record={root.record} isRoot isSelected={selectedUserID === root.record.userID} {text} {selectRecord} />
						</div>
					{/each}
				</div>

				{#if model.columns.length > 0}
					<div class="relative h-20 text-border" aria-hidden="true" data-testid="orgchart-connector-layer">
						<svg class="absolute inset-0 h-full w-full" viewBox={`0 0 ${connectorWidth} 96`} preserveAspectRatio="none">
							{#each rootConnectorEdges as edge}
								<path d={orgchartEdgePath(edge)} fill="none" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke" />
							{/each}
						</svg>
					</div>
					<div class="grid w-full" style={teamGridStyle} data-testid="orgchart-team-grid">
						{#each teamLayouts as layout (layout.column.id)}
							<div class="relative min-w-0" style={`width: ${layout.width}px; height: ${layout.height}px`} data-testid={`orgchart-team-column-${layout.column.id}`}>
								<svg class="absolute inset-0 size-full text-border" viewBox={`0 0 ${layout.width} ${layout.height}`} preserveAspectRatio="none" aria-hidden="true">
									{#each layout.edges as edge}
										<path d={orgchartEdgePath(edge)} fill="none" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke" />
									{/each}
								</svg>
								<header class={['absolute grid h-14 content-center rounded-t-md border px-3 text-center', layout.column.treeRoots.length === 1 && 'border-b-0', teamTone()]} style={`left: ${layout.headerX}px; top: 0; width: ${layout.headerWidth}px`}>
									<h2 class="truncate text-base font-semibold">{layout.column.name}</h2>
								</header>
								{#each layout.nodes as node (node.record.userID)}
									<div class="absolute" style={`left: ${node.x}px; top: ${node.y}px; width: ${orgchartPersonNodeWidth}px`} data-testid={`orgchart-tree-node-${node.record.userID}`}>
										<OrgchartPersonNode
											record={node.record}
											isAttachedToHeader={node.isAttachedToHeader}
											isSelected={selectedUserID === node.record.userID}
											{text}
											{selectRecord}
										/>
									</div>
								{/each}
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	</div>
</section>
