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

	let { nodes, edges }: { nodes: MemoryNetworkNode[]; edges: MemoryNetworkEdge[] } = $props();

	let containerElement: HTMLDivElement;
	let graph: ForceGraphDefault<ForceGraphNode, ForceGraphLink> | null = null;

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
		fitGraphToContainer();
	});

	onMount(() => {
		let isMounted = true;
		let resizeObserver: ResizeObserver | null = null;

		setupGraph();

		return () => {
			isMounted = false;
			resizeObserver?.disconnect();
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
				.linkColor(() => 'rgba(100,116,139,0.35)')
				.linkWidth((link: ForceGraphLink) => Math.max(1, Number(link.weight || 1)))
				.backgroundColor('rgba(0,0,0,0)')
				.graphData(graphData);

			resizeObserver = new ResizeObserver(() => resizeGraph());
			resizeObserver.observe(containerElement);
			resizeGraph();
			fitGraphToContainer();
		}
	});

	function resizeGraph() {
		if (!graph || !containerElement) return;
		graph.width(containerElement.clientWidth);
		graph.height(containerElement.clientHeight);
		fitGraphToContainer();
	}

	function fitGraphToContainer() {
		if (!graph) return;
		window.setTimeout(() => graph?.zoomToFit(250, 56), 100);
	}

	function nodeLabel(node: ForceGraphNode) {
		return [node.label, node.kind, node.scopeType, node.status].filter(Boolean).join(' · ');
	}

	function nodeColor(node: ForceGraphNode) {
		if (node.kind === 'namespace') return '#0f766e';
		if (node.kind === 'fact') return '#7c3aed';
		if (node.status === 'failed') return '#dc2626';
		return '#475569';
	}

	function nodeValue(node: ForceGraphNode) {
		if (node.kind === 'namespace') return 9;
		if (node.kind === 'fact') return 5;
		return 4;
	}
</script>

<div bind:this={containerElement} class="h-[420px] w-full min-w-0 overflow-hidden rounded-lg border bg-muted/20 sm:h-[520px]"></div>
