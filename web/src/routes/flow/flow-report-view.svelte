<script lang="ts">
	import FlowReportCard from './report/flow-report-card.svelte';
	import type { FlowReportSections } from './report/flow-report-data';

	type Props = {
		sections: FlowReportSections;
	};

	let { sections }: Props = $props();
</script>

<section class="grid min-w-0 gap-4 lg:grid-cols-2">
	<div class="grid min-w-0 gap-4">
		<FlowReportCard section={sections.weeklyStatus} />
		<FlowReportCard section={sections.businessDistance} />
	</div>
	<div class="min-w-0">
		<FlowReportCard section={sections.memberDistance} />
	</div>
	<FlowReportCard section={sections.weeklyDistanceTrend} />
	<FlowReportCard section={sections.monthlyDistanceTrend} />
</section>
