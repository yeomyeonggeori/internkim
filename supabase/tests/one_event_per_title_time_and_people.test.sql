begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
	('30000000-0000-0000-0000-000000000001', 'calendar-one@example.com'),
	('30000000-0000-0000-0000-000000000002', 'calendar-two@example.com');

insert into public.company (
	id, name, slug, country, locale, timezone, work_hours, minimum_daily_minutes, rules
) values (
	'30000000-0000-0000-0000-000000000000', 'Calendar', 'calendar', 'KR', 'ko', 'Asia/Seoul',
	'[[[],[],[],[],[],null,null]]', 480, '{}'
);

insert into public.member (id, company_id, email, user_id, status) values
	(
		'30000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000000',
		'calendar-one@example.com', '30000000-0000-0000-0000-000000000001', 'active'
	),
	(
		'30000000-0000-0000-0000-000000000012', '30000000-0000-0000-0000-000000000000',
		'calendar-two@example.com', '30000000-0000-0000-0000-000000000002', 'active'
	);

insert into public.task (id, company_id, title, starts_at, ends_at, is_event) values
	(
		'30000000-0000-0000-0000-0000000000a1', '30000000-0000-0000-0000-000000000000',
		'마켓컬리 CMO 미팅', '2026-08-20T10:00:00Z', '2026-08-20T11:00:00Z', true
	);
insert into public.task_participant (task_id, member_id) values
	('30000000-0000-0000-0000-0000000000a1', '30000000-0000-0000-0000-000000000011');

prepare a_second_copy as
	insert into public.task (company_id, title, starts_at, ends_at, is_event) values
		(
			'30000000-0000-0000-0000-000000000000',
			'마켓컬리 CMO 미팅', '2026-08-20T10:00:00Z', '2026-08-20T11:00:00Z', true
		);

select lives_ok(
	'a_second_copy',
	'a copy with nobody on it yet is not the same event, because the people are what differ'
);

-- The check is deferred so a writer can build one event's people up without an
-- intermediate set colliding with another event. Nothing commits inside a pgTAP
-- transaction, so the test asks for the check by hand.
insert into public.task_participant (task_id, member_id)
select id, '30000000-0000-0000-0000-000000000011'
from public.task
where title = '마켓컬리 CMO 미팅' and id <> '30000000-0000-0000-0000-0000000000a1';

select throws_ok(
	'set constraints all immediate',
	23505,
	null,
	'putting the same person on it makes it the same event, and the record refuses'
);

set constraints all deferred;

insert into public.task (id, company_id, title, starts_at, ends_at, is_event) values
	(
		'30000000-0000-0000-0000-0000000000a3', '30000000-0000-0000-0000-000000000000',
		'같은 시간 다른 사람', '2026-08-20T10:00:00Z', '2026-08-20T11:00:00Z', true
	),
	(
		'30000000-0000-0000-0000-0000000000a4', '30000000-0000-0000-0000-000000000000',
		'같은 시간 다른 사람', '2026-08-20T10:00:00Z', '2026-08-20T11:00:00Z', true
	);

select lives_ok(
	$$
		insert into public.task_participant (task_id, member_id) values
			('30000000-0000-0000-0000-0000000000a3', '30000000-0000-0000-0000-000000000011'),
			('30000000-0000-0000-0000-0000000000a4', '30000000-0000-0000-0000-000000000012')
	$$,
	'the same title at the same time with different people is two events, not one'
);

select lives_ok(
	$$
		insert into public.task (company_id, title, starts_at, ends_at, is_event) values
			(
				'30000000-0000-0000-0000-000000000000',
				'마켓컬리 CMO 미팅', '2026-08-21T10:00:00Z', '2026-08-21T11:00:00Z', true
			)
	$$,
	'the same event on another day is another event'
);

select lives_ok(
	$$
		insert into public.task (company_id, title, is_event) values
			('30000000-0000-0000-0000-000000000000', '같은 제목 업무', false),
			('30000000-0000-0000-0000-000000000000', '같은 제목 업무', false)
	$$,
	'a task is not an event, so two of them by the same name are allowed'
);

select * from finish();
rollback;
