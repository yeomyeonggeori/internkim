<script lang="ts">
import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
import DefinitionBadge from '$lib/components/definition-badge.svelte';
import LocationLabel from '../shared/location-label.svelte';
import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
import { attendancePreviewEnabled } from '$lib/attendance/attendance-preview';
const people = Array.from({length: 16}, (_,i) => ({name: '샘플'+i, email:'stack'+i+'@example.com',memberID:'stack-'+i}));
</script>
{#if attendancePreviewEnabled()}
<div class="flex flex-col gap-8 p-6"><h1>공용 Avatar Group 검사</h1><div data-testid="stack-one"><PersonAvatarStack people={people.slice(0,5)} max={4} /></div><div data-testid="stack-many"><PersonAvatarStack {people} max={4} /></div><div data-testid="stack-none"><PersonAvatarStack people={people.slice(0,3)} max={3} /></div><div class="flex flex-wrap gap-3" data-testid="color-tests"><ColoredOutlineBadge color="#fff3a0">밝은 노랑</ColoredOutlineBadge><ColoredOutlineBadge color="#08154a">짙은 파랑</ColoredOutlineBadge><ColoredOutlineBadge>색 없음</ColoredOutlineBadge><DefinitionBadge label="업무 종류" color="#ef8060" /><LocationLabel name="등록된 근무지" color="#008080" /></div></div>
{/if}