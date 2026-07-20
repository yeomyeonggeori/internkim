<script lang="ts">
	import type { FlowSummary } from './flow-types';
	import { flowText } from './text';
	import FlowMemberScoreCard from './report/flow-member-score-card.svelte';
	import FlowReportCard from './report/flow-report-card.svelte';
	import type { FlowReportSections } from './report/flow-report-data';

	type FlowReportText = typeof flowText.ko.report;

	type Props = {
		sections: FlowReportSections;
		summary: FlowSummary | null;
		text: FlowReportText;
	};

	let { sections, summary, text }: Props = $props();
</script>

<section class="grid min-w-0 gap-4 lg:grid-cols-2">
	<div class="grid min-w-0 gap-4">
		<FlowReportCard section={sections.weeklyStatus} />
		<FlowReportCard section={sections.businessDistance} />
	</div>
	<div class="min-w-0">
		<FlowMemberScoreCard section={sections.memberDistance} {summary} {text} />
	</div>
	<FlowReportCard section={sections.weeklyDistanceTrend} />
	<FlowReportCard section={sections.monthlyDistanceTrend} />
</section>
