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
		<FlowReportCard section={sections.weeklyStatus} definitions={summary?.definitions} />
		<FlowReportCard section={sections.businessDistance} definitions={summary?.definitions} />
	</div>
	<div class="min-w-0">
		<FlowMemberScoreCard section={sections.memberDistance} {summary} {text} />
	</div>
	<FlowReportCard section={sections.weeklyDistanceTrend} definitions={summary?.definitions} />
	<FlowReportCard section={sections.monthlyDistanceTrend} definitions={summary?.definitions} />
</section>
