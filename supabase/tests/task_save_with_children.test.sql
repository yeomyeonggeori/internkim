begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
	('44000000-0000-0000-0000-000000000001', 'new-parent@example.test'),
	('44000000-0000-0000-0000-000000000002', 'new-parent-colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
	('44000000-0000-0000-0000-0000000000a0', 'New Parent', 'new-parent', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
	('44000000-0000-0000-0000-0000000000a1', '44000000-0000-0000-0000-0000000000a0', 'new-parent@example.test', '44000000-0000-0000-0000-000000000001', 'active'),
	('44000000-0000-0000-0000-0000000000a2', '44000000-0000-0000-0000-0000000000a0', 'new-parent-colleague@example.test', '44000000-0000-0000-0000-000000000002', 'active');

insert into public.task (id, company_id, title, parent_task_id) values
	('44000000-0000-0000-0000-000000000101', '44000000-0000-0000-0000-0000000000a0', 'Earlier parent', null),
	('44000000-0000-0000-0000-000000000102', '44000000-0000-0000-0000-0000000000a0', 'Child A', null),
	('44000000-0000-0000-0000-000000000103', '44000000-0000-0000-0000-0000000000a0', 'Child B', null),
	('44000000-0000-0000-0000-000000000104', '44000000-0000-0000-0000-0000000000a0', 'Taken child', '44000000-0000-0000-0000-000000000101');

insert into public.task_participant (task_id, member_id) values
	('44000000-0000-0000-0000-000000000101', '44000000-0000-0000-0000-0000000000a1'),
	('44000000-0000-0000-0000-000000000102', '44000000-0000-0000-0000-0000000000a1'),
	('44000000-0000-0000-0000-000000000103', '44000000-0000-0000-0000-0000000000a1'),
	('44000000-0000-0000-0000-000000000104', '44000000-0000-0000-0000-0000000000a1');

set local role authenticated;
select set_config('request.jwt.claim.sub', '44000000-0000-0000-0000-000000000001', true);

select lives_ok(
	$$select public.task_save_with_children(
		jsonb_build_object(
			'target_title', 'Shared parent',
			'target_status', 'planned',
			'target_participant_ids', jsonb_build_array(
				'44000000-0000-0000-0000-0000000000a1',
				'44000000-0000-0000-0000-0000000000a2'
			)
		),
		array['44000000-0000-0000-0000-000000000102'::uuid]
	)$$,
	'save with children: a new task shared with a colleague takes its child'
);

select is(
	(
		select parent.title
		from public.task as child
		join public.task as parent on parent.id = child.parent_task_id
		where child.id = '44000000-0000-0000-0000-000000000102'
	),
	'Shared parent',
	'save with children: the child stores the new task as its parent'
);

select throws_ok(
	$$select public.task_save_with_children(
		jsonb_build_object(
			'target_title', 'Refused shared parent',
			'target_status', 'planned',
			'target_participant_ids', jsonb_build_array(
				'44000000-0000-0000-0000-0000000000a1',
				'44000000-0000-0000-0000-0000000000a2'
			)
		),
		array['44000000-0000-0000-0000-000000000103'::uuid, '44000000-0000-0000-0000-000000000104'::uuid]
	)$$,
	'23514',
	'every selected child task must be available to the authenticated member and have no parent',
	'save with children: a child that cannot move refuses the whole save'
);

select is(
	(select count(*)::integer from public.task where title = 'Refused shared parent'),
	0,
	'save with children: a refused save adds no task, even one its maker could not delete'
);

select throws_ok(
	$$select public.task_save_with_children(
		jsonb_build_object('target_task_id', '44000000-0000-0000-0000-000000000101', 'target_title', 'Earlier parent'),
		array['44000000-0000-0000-0000-000000000103'::uuid]
	)$$,
	'23514',
	'only a new task takes its children as it is saved',
	'save with children: an existing task links children through task_children_link'
);

select * from finish();
rollback;
