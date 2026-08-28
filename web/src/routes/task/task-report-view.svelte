<script lang="ts">
	import type { TaskSummary } from './task-types';
	import { taskText } from './text';
	import TaskMemberScoreCard from './report/task-member-score-card.svelte';
	import TaskReportCard from './report/task-report-card.svelte';
	import type { TaskReportSections } from './report/task-report-data';

	type TaskReportText = typeof taskText.ko.report;

	type Props = {
		sections: TaskReportSections;
		summary: TaskSummary | null;
		text: TaskReportText;
	};

	let { sections, summary, text }: Props = $props();
</script>

<section class="grid min-w-0 gap-4 lg:grid-cols-2">
	<div class="grid min-w-0 gap-4">
		<TaskReportCard section={sections.weeklyStatus} definitions={summary?.definitions} />
		<TaskReportCard section={sections.businessDistance} definitions={summary?.definitions} />
	</div>
	<div class="min-w-0">
		<TaskMemberScoreCard section={sections.memberDistance} {summary} {text} />
	</div>
	<TaskReportCard section={sections.weeklyDistanceTrend} definitions={summary?.definitions} />
	<TaskReportCard section={sections.monthlyDistanceTrend} definitions={summary?.definitions} />
</section>
