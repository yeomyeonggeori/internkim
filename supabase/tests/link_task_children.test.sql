begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

insert into auth.users (id, email) values
	('43000000-0000-0000-0000-000000000001', 'link-children@example.test'),
	('43000000-0000-0000-0000-000000000002', 'other-parent@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('43000000-0000-0000-0000-0000000000a0', 'Link Children', 'link-children', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
	(
    '43000000-0000-0000-0000-0000000000a1',
    '43000000-0000-0000-0000-0000000000a0',
    'link-children@example.test',
    '43000000-0000-0000-0000-000000000001',
		'active'
	),
	(
		'43000000-0000-0000-0000-0000000000a2',
		'43000000-0000-0000-0000-0000000000a0',
		'other-parent@example.test',
		'43000000-0000-0000-0000-000000000002',
		'active'
	);

insert into public.task (id, company_id, title, parent_task_id) values
  ('43000000-0000-0000-0000-000000000101', '43000000-0000-0000-0000-0000000000a0', 'Parent', null),
  ('43000000-0000-0000-0000-000000000102', '43000000-0000-0000-0000-0000000000a0', 'Child A', null),
  ('43000000-0000-0000-0000-000000000103', '43000000-0000-0000-0000-0000000000a0', 'Child B', null),
  ('43000000-0000-0000-0000-000000000104', '43000000-0000-0000-0000-0000000000a0', 'Unavailable child', '43000000-0000-0000-0000-000000000101'),
	('43000000-0000-0000-0000-000000000105', '43000000-0000-0000-0000-0000000000a0', 'Available child', null),
	('43000000-0000-0000-0000-000000000106', '43000000-0000-0000-0000-0000000000a0', 'Other parent', null),
	('43000000-0000-0000-0000-000000000107', '43000000-0000-0000-0000-0000000000a0', 'Unauthorized child', null);

insert into public.task_participant (task_id, member_id) values
  ('43000000-0000-0000-0000-000000000101', '43000000-0000-0000-0000-0000000000a1'),
  ('43000000-0000-0000-0000-000000000102', '43000000-0000-0000-0000-0000000000a1'),
  ('43000000-0000-0000-0000-000000000103', '43000000-0000-0000-0000-0000000000a1'),
  ('43000000-0000-0000-0000-000000000104', '43000000-0000-0000-0000-0000000000a1'),
	('43000000-0000-0000-0000-000000000105', '43000000-0000-0000-0000-0000000000a1'),
	('43000000-0000-0000-0000-000000000106', '43000000-0000-0000-0000-0000000000a2'),
	('43000000-0000-0000-0000-000000000107', '43000000-0000-0000-0000-0000000000a2');

set local role authenticated;
select set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);

select lives_ok(
  $$select public.task_children_link(
    '43000000-0000-0000-0000-000000000101',
    array[
      '43000000-0000-0000-0000-000000000102'::uuid,
      '43000000-0000-0000-0000-000000000103'::uuid
    ]
  )$$,
  'link task children: all selected children are linked together'
);

select is(
  (
    select count(*)::integer
    from public.task
    where parent_task_id = '43000000-0000-0000-0000-000000000101'
      and id in (
        '43000000-0000-0000-0000-000000000102',
        '43000000-0000-0000-0000-000000000103'
      )
  ),
  2,
  'link task children: both selected children store the parent'
);

select throws_ok(
  $$select public.task_children_link(
    '43000000-0000-0000-0000-000000000101',
    array[
      '43000000-0000-0000-0000-000000000105'::uuid,
      '43000000-0000-0000-0000-000000000104'::uuid
    ]
	)$$,
	'23514',
	'every selected child task must be available to the authenticated member and have no parent',
  'link task children: unavailable children reject the whole call'
);

select is(
  (
    select count(*)::integer
    from public.task
    where id = '43000000-0000-0000-0000-000000000105'
      and parent_task_id is null
  ),
  1,
	'link task children: a rejected batch leaves available children unchanged'
);

select throws_ok(
	$$select public.task_parent_set(
		'43000000-0000-0000-0000-000000000105',
		'43000000-0000-0000-0000-000000000106'
	)$$,
	'42501',
	'only a parent task participant or company admin can use that parent',
	'set task parent: a participant cannot use a colleague-only parent'
);

select throws_ok(
	$$select public.task_children_link(
		'43000000-0000-0000-0000-000000000101',
		array['43000000-0000-0000-0000-000000000107'::uuid]
	)$$,
	'23514',
	'every selected child task must be available to the authenticated member and have no parent',
	'link task children: a participant cannot link a colleague-only child'
);

select is(
	(
		select count(*)::integer
		from public.task
		where id = '43000000-0000-0000-0000-000000000107'
			and parent_task_id is null
	),
	1,
	'link task children: an unauthorized child remains unchanged'
);

select throws_ok(
	$$select public.task_save(
	  target_task_id => null,
	  target_title => 'Atomic child',
	  target_status => 'todo',
	  target_note => null,
	  target_business => null,
	  target_type => null,
	  target_size => null,
	  target_starts_at => null,
	  target_ends_at => null,
	  target_write_dates => true,
	  target_participant_ids => array['43000000-0000-0000-0000-0000000000a1'::uuid],
	  target_parent_task_id => '43000000-0000-0000-0000-000000000106',
	  target_is_event => false
	)$$,
	'42501',
	'only a parent task participant or company admin can use that parent',
	'save flow task: parent authorization failure rolls back child creation'
);

select is(
	(
		select count(*)::integer
		from public.task
		where title = 'Atomic child'
	),
	0,
	'save flow task: failed parent linking leaves no orphan child'
);

reset role;

select * from finish();
rollback;
