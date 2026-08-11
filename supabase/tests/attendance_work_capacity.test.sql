begin;
create extension if not exists pgtap with schema extensions;
select plan(2);

insert into auth.users (id, email) values
	('10000000-0000-0000-0000-000000000001', 'capacity-a@example.com'),
	('20000000-0000-0000-0000-000000000001', 'capacity-b@example.com');

insert into public.company (
	id, name, slug, country, locale, timezone, work_hours, minimum_daily_minutes, attendance_work_mode
) values
	(
		'10000000-0000-0000-0000-000000000000', 'Capacity A', 'capacity-a', 'KR', 'ko', 'Asia/Seoul',
		'[[[],[],[],[],[],null,null]]', 480, 'fixed'
	),
	(
		'20000000-0000-0000-0000-000000000000', 'Capacity B', 'capacity-b', 'US', 'en-US', 'America/New_York',
		'[[[],[],[],[],[],null,null]]', 420, 'autonomous'
	);

insert into public.member (id, company_id, email, user_id, status) values
	(
		'10000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000000',
		'capacity-a@example.com', '10000000-0000-0000-0000-000000000001', 'active'
	),
	(
		'10000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000000',
		'capacity-a-colleague@example.com', null, 'active'
	),
	(
		'20000000-0000-0000-0000-000000000011', '20000000-0000-0000-0000-000000000000',
		'capacity-b@example.com', '20000000-0000-0000-0000-000000000001', 'active'
	);

set local role authenticated;
select set_config('request.jwt.claim.sub', '10000000-0000-0000-0000-000000000001', true);

select is(
	(select count(*) from public.attendance_work_policies()),
	2::bigint,
	'attendance work policies resolve every visible colleague through member fallback functions'
);

select is(
	(select min(work_mode) from public.attendance_work_policies()),
	'fixed',
	'attendance work policies carry the company actual work mode'
);

select * from finish();
rollback;
