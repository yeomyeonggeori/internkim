begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
	('45000000-0000-0000-0000-000000000001', 'finishing-parent@example.test'),
	('45000000-0000-0000-0000-000000000002', 'finishing-colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
	('45000000-0000-0000-0000-0000000000a0', 'Finishing Parent', 'finishing-parent', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
	('45000000-0000-0000-0000-0000000000a1', '45000000-0000-0000-0000-0000000000a0', 'finishing-parent@example.test', '45000000-0000-0000-0000-000000000001', 'active'),
	('45000000-0000-0000-0000-0000000000a2', '45000000-0000-0000-0000-0000000000a0', 'finishing-colleague@example.test', '45000000-0000-0000-0000-000000000002', 'active');

insert into public.task (id, company_id, title, status, parent_task_id) values
	('45000000-0000-0000-0000-000000000101', '45000000-0000-0000-0000-0000000000a0', 'Parent', 'in_progress', null),
	('45000000-0000-0000-0000-000000000102', '45000000-0000-0000-0000-0000000000a0', 'Open child', 'planned', '45000000-0000-0000-0000-000000000101'),
	('45000000-0000-0000-0000-000000000103', '45000000-0000-0000-0000-0000000000a0', 'Stopped child', 'stopped', '45000000-0000-0000-0000-000000000101'),
	('45000000-0000-0000-0000-000000000104', '45000000-0000-0000-0000-0000000000a0', 'Grandchild', 'in_progress', '45000000-0000-0000-0000-000000000102'),
	('45000000-0000-0000-0000-000000000201', '45000000-0000-0000-0000-0000000000a0', 'Parent over a colleague', 'in_progress', null),
	('45000000-0000-0000-0000-000000000202', '45000000-0000-0000-0000-0000000000a0', 'Colleague''s child', 'planned', '45000000-0000-0000-0000-000000000201');

insert into public.task_participant (task_id, member_id) values
	('45000000-0000-0000-0000-000000000101', '45000000-0000-0000-0000-0000000000a1'),
	('45000000-0000-0000-0000-000000000102', '45000000-0000-0000-0000-0000000000a1'),
	('45000000-0000-0000-0000-000000000103', '45000000-0000-0000-0000-0000000000a1'),
	('45000000-0000-0000-0000-000000000104', '45000000-0000-0000-0000-0000000000a1'),
	('45000000-0000-0000-0000-000000000201', '45000000-0000-0000-0000-0000000000a1'),
	('45000000-0000-0000-0000-000000000202', '45000000-0000-0000-0000-0000000000a2');

create function pg_temp.completing(target uuid, title text) returns jsonb language sql as $$
	select jsonb_build_object(
		'target_task_id', target,
		'target_title', title,
		'target_status', 'completed',
		'target_write_dates', false,
		'target_participant_ids', jsonb_build_array('45000000-0000-0000-0000-0000000000a1')
	);
$$;

set local role authenticated;
select set_config('request.jwt.claim.sub', '45000000-0000-0000-0000-000000000001', true);

select throws_ok(
	$$select public.task_save_completing_children(pg_temp.completing('45000000-0000-0000-0000-000000000101', 'Parent') || '{"target_status": "in_progress"}')$$,
	'23514',
	'only a task being completed completes the work under it',
	'completing children: only a write that completes the parent completes the work under it');

select throws_ok(
	$$select public.task_save_completing_children(pg_temp.completing('45000000-0000-0000-0000-000000000201', 'Parent over a colleague'))$$,
	'42501',
	'work under this task that the caller cannot change is still unfinished',
	'completing children: a child the caller cannot change stops the whole write');

select is(
	(select status::text from public.task where id = '45000000-0000-0000-0000-000000000201'),
	'in_progress',
	'completing children: a stopped write leaves the parent as it was');

select is(
	public.task_save_completing_children(pg_temp.completing('45000000-0000-0000-0000-000000000101', 'Parent')),
	'45000000-0000-0000-0000-000000000101'::uuid,
	'completing children: the parent is saved');

select is(
	(select array_agg(status::text order by id) from public.task where id::text like '45000000-0000-0000-0000-00000000010%'),
	array['completed', 'completed', 'stopped', 'completed'],
	'completing children: the parent and every unfinished task under it, at any depth, are completed, and a stopped one stays stopped');

select is(
	(select count(*)::integer from public.task where id::text like '45000000-0000-0000-0000-00000000010%' and status = 'completed' and ends_at is null),
	0,
	'completing children: each completed task carries an end, as any completion does');

select * from finish();
rollback;
