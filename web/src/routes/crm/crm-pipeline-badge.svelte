<script lang="ts">
	import ColorMarkerBadge from '$lib/components/color-marker-badge.svelte';
	import { crmLabel } from './crm-labels';
	import type { CRMOpportunity, CRMOrganization, CRMPipeline } from './crm-types';
	import { getProgressKind } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		opportunity: CRMOpportunity;
		organization: CRMOrganization | undefined;
		pipelines: CRMPipeline[];
		text: CRMText;
		class?: string;
	};

	let { opportunity, organization, pipelines, text, class: className }: Props = $props();

	const kind = $derived(getProgressKind(opportunity, organization));
	const definition = $derived(pipelines.find((pipeline) => pipeline.pipeline === kind));
	const label = $derived(definition?.label ?? crmLabel(text.progressKinds, kind));
</script>

<ColorMarkerBadge {label} color={definition?.color} class={className} />
