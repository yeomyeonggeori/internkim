<!-- 업무 목록 테이블의 크기 배지 셀을 렌더링한다. -->
<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { sizeBadgeClass } from './flow-style';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
	};

	let { task }: Props = $props();
</script>

<Badge class={sizeBadgeClass(task.size)}>{task.size}</Badge>
