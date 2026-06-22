<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import type ForceGraphDefault from 'force-graph';
	import type { LinkObject, NodeObject } from 'force-graph';

	type MemoryNetworkNode = {
		nodeID: string;
		label: string;
		kind: string;
		scopeType?: string;
		status?: string;
	};

	type MemoryNetworkEdge = {
		sourceID: string;
		targetID: string;
		weight?: number;
	};

	type ForceGraphNode = MemoryNetworkNode & NodeObject & {
		id: string;
	};

	type ForceGraphLink = LinkObject<ForceGraphNode> & {
		source: string;
		target: string;
		weight: number;
	};

	let {
		nodes,
		edges,
		selectedNodeID = null,
		onNodeSelect,
		onClearSelection
	}: {
		nodes: MemoryNetworkNode[];
		edges: MemoryNetworkEdge[];
		selectedNodeID?: string | null;
		onNodeSelect?: (nodeID: string) => void;
		onClearSelection?: () => void;
	} = $props();

	let containerElement: HTMLDivElement;
	let graph: ForceGraphDefault<ForceGraphNode, ForceGraphLink> | null = null;
	let graphFitTimeoutID: number | null = null;

	const graphFitDelayMillisecond = 120;
	const graphFitDurationMillisecond = 250;
	const graphFitPaddingPixel = 48;
	const graphLayoutCooldownTickCount = 120;
	const graphLayoutWarmupTickCount = 30;

	const graphData = $derived({
		nodes: nodes.map((node) => ({ ...node, id: node.nodeID })),
		links: edges.map((edge) => ({
			source: edge.sourceID,
			target: edge.targetID,
			weight: edge.weight ?? 1
		}))
	});

	$effect(() => {
		if (!graph) return;
		graph.graphData(graphData);
		syncGraphSize();
		graph.d3ReheatSimulation();
		scheduleGraphFit();
	});

	$effect(() => {
		selectedNodeID;
		graph?.nodeColor((node: ForceGraphNode) => nodeColor(node)).nodeVal((node: ForceGraphNode) => nodeValue(node));
	});

	onMount(() => {
		let isMounted = true;
		let resizeObserver: ResizeObserver | null = null;

		setupGraph();

		return () => {
			isMounted = false;
			resizeObserver?.disconnect();
			clearGraphFit();
			graph?.pauseAnimation();
			graph = null;
		};

		async function setupGraph() {
			if (!browser || !containerElement) return;

			const forceGraphModule: { default: typeof ForceGraphDefault } = await import('force-graph');
			if (!isMounted) return;
			graph = new forceGraphModule.default<ForceGraphNode, ForceGraphLink>(containerElement)
				.nodeId('id')
				.nodeLabel((node: ForceGraphNode) => nodeLabel(node))
				.nodeColor((node: ForceGraphNode) => nodeColor(node))
				.nodeVal((node: ForceGraphNode) => nodeValue(node))
				.onNodeClick((node: ForceGraphNode) => onNodeSelect?.(node.id))
				.onBackgroundClick(() => onClearSelection?.())
				.linkColor(() => 'rgba(100,116,139,0.35)')
				.linkWidth((link: ForceGraphLink) => Math.max(1, Number(link.weight || 1)))
				.warmupTicks(graphLayoutWarmupTickCount)
				.cooldownTicks(graphLayoutCooldownTickCount)
				.onEngineStop(() => scheduleGraphFit())
				.backgroundColor('rgba(0,0,0,0)')
				.graphData(graphData);

			resizeObserver = new ResizeObserver(() => {
				syncGraphSize();
				scheduleGraphFit();
			});
			resizeObserver.observe(containerElement);
			syncGraphSize();
			scheduleGraphFit();
		}
	});

	function syncGraphSize(): void {
		if (!graph || !containerElement) return;
		if (containerElement.clientWidth === 0 || containerElement.clientHeight === 0) return;
		graph.width(containerElement.clientWidth).height(containerElement.clientHeight);
	}

	function scheduleGraphFit(): void {
		if (!browser || !graph) return;
		clearGraphFit();
		graphFitTimeoutID = window.setTimeout(() => {
			graph?.zoomToFit(graphFitDurationMillisecond, graphFitPaddingPixel);
			graphFitTimeoutID = null;
		}, graphFitDelayMillisecond);
	}

	function clearGraphFit(): void {
		if (graphFitTimeoutID === null) return;
		window.clearTimeout(graphFitTimeoutID);
		graphFitTimeoutID = null;
	}

	function nodeLabel(node: ForceGraphNode): string {
		return [node.label, node.kind, node.scopeType, node.status].filter(Boolean).join(' · ');
	}

	function nodeColor(node: ForceGraphNode): string {
		if (node.id === selectedNodeID) return '#f59e0b';
		if (node.kind === 'namespace') return '#0f766e';
		if (node.kind === 'fact') return '#7c3aed';
		if (node.status === 'failed') return '#dc2626';
		return '#475569';
	}

	function nodeValue(node: ForceGraphNode): number {
		const baseValue = node.kind === 'namespace' ? 9 : node.kind === 'fact' ? 5 : 4;
		return node.id === selectedNodeID ? baseValue * 1.6 : baseValue;
	}
</script>

<div
	bind:this={containerElement}
	class="h-[min(58svh,520px)] min-h-[360px] w-full min-w-0 overflow-hidden rounded-lg border bg-muted/20"
></div>
