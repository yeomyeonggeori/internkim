<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import MaximizeIcon from '@lucide/svelte/icons/maximize';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import type { UserRecord } from '../admin/admin-types';
	import type { OrgchartCanvasModel } from './orgchart-directory-model';
	import {
		orgchartBoardMinimumWidth,
		orgchartColumnCenterPositions,
		orgchartDistributedColumnSpace,
		orgchartEdgePath,
		orgchartPersonNodeWidth,
		orgchartTeamLayout
	} from './orgchart-layout';
	import OrgchartPersonNode from './orgchart-person-node.svelte';
	import type { orgchartDirectoryText } from './text';

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
	let centeredBoardKey = $state('');

	const zoomLabel = $derived(`${zoom}%`);
	const canvasScale = $derived(zoom / 100);
	const teamLayouts = $derived(model.columns.map((column) => orgchartTeamLayout(column)));
	const columnWidths = $derived(teamLayouts.map((layout) => layout.width));
	const boardMinimumWidth = $derived(orgchartBoardMinimumWidth(columnWidths));
	const boardWidth = $derived(Math.max(boardMinimumWidth, viewportWidth));
	const columnSpace = $derived(orgchartDistributedColumnSpace(columnWidths, boardWidth));
	const connectorWidth = $derived(boardWidth);
	const connectorCenter = $derived(connectorWidth / 2);
	const connectorPositions = $derived(orgchartColumnCenterPositions(columnWidths, columnSpace, boardWidth));
	const connectorStart = $derived(connectorPositions[0] ?? connectorCenter);
	const connectorEnd = $derived(connectorPositions[connectorPositions.length - 1] ?? connectorCenter);
	const teamGridStyle = $derived(`grid-template-columns: ${columnWidths.map((width) => `${width}px`).join(' ')}; gap: ${columnSpace}px; width: ${boardWidth}px; justify-content: ${columnWidths.length === 1 ? 'center' : 'start'};`);
	const boardStyle = $derived(`min-width: ${boardWidth}px; width: ${boardWidth}px; zoom: ${canvasScale};`);

	function teamTone(): string {
		return 'border-slate-200 bg-slate-50 text-slate-800 dark:border-slate-800 dark:bg-slate-950/30 dark:text-slate-200';
	}

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
		<Button variant="ghost" size="icon" aria-label={text.zoomOut} onclick={() => setZoom(Math.max(70, zoom - 10))}>
			<MinusIcon class="size-4" />
		</Button>
		<div class="min-w-16 px-3 text-center text-sm font-medium">{zoomLabel}</div>
		<Button variant="ghost" size="icon" aria-label={text.zoomIn} onclick={() => setZoom(Math.min(130, zoom + 10))}>
			<PlusIcon class="size-4" />
		</Button>
		<Button variant="ghost" size="icon" aria-label={text.zoomFit} onclick={() => setZoom(100)}>
			<MaximizeIcon class="size-4" />
		</Button>
	</div>

	<div bind:this={scrollElement} class="min-h-0 flex-1 overflow-auto pb-8" data-testid="orgchart-canvas-scroll">
		<div bind:clientWidth={viewportWidth} class="grid min-h-full w-full content-start justify-items-center pt-3">
			<div class="grid min-h-full content-start justify-items-stretch" style={boardStyle}>
				<div class="flex justify-center">
					{#each model.roots as root (root.userID)}
						<OrgchartPersonNode record={root} isRoot isSelected={selectedUserID === root.userID} {text} {selectRecord} />
					{/each}
				</div>

				{#if model.columns.length > 0}
					<div class="relative h-20 text-border" aria-hidden="true" data-testid="orgchart-connector-layer">
						<svg class="absolute inset-0 h-full w-full" viewBox={`0 0 ${connectorWidth} 96`} preserveAspectRatio="none">
							<line x1={connectorCenter} y1="0" x2={connectorCenter} y2="48" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke" />
							<line x1={connectorStart} y1="48" x2={connectorEnd} y2="48" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke" />
							{#each model.columns as column, index (column.id)}
								<line x1={connectorPositions[index]} y1="48" x2={connectorPositions[index]} y2="96" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke" />
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
								<header class={['absolute grid h-14 content-center rounded-t-md border border-b-0 px-3 text-center', teamTone()]} style={`left: ${layout.headerX}px; top: 0; width: ${orgchartPersonNodeWidth}px`}>
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
