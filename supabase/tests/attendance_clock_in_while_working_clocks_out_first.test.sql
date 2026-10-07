begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
	('48000000-0000-0000-0000-000000000001', 'working-mover@example.test'),
	('48000000-0000-0000-0000-000000000002', 'hand-writer@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	('48000000-0000-0000-0000-000000000000', 'Sample Mover', 'sample-mover', 'KR', 'ko', 'Asia/Seoul',
	 '[{"name":"Office"},{"name":"Home"}]', '[[[],[],[],[],[],null,null]]', 480);

insert into public.member (id, company_id, email, user_id, status) values
	('48000000-0000-0000-0000-000000000011', '48000000-0000-0000-0000-000000000000',
	 'working-mover@example.test', '48000000-0000-0000-0000-000000000001', 'active'),
	('48000000-0000-0000-0000-000000000012', '48000000-0000-0000-0000-000000000000',
	 'hand-writer@example.test', '48000000-0000-0000-0000-000000000002', 'active');

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	('48000000-0000-0000-0000-000000000101', '48000000-0000-0000-0000-000000000011',
	 'clock_in', 'Home', now() - interval '3 hours'),
	('48000000-0000-0000-0000-000000000201', '48000000-0000-0000-0000-000000000012',
	 'clock_in', 'Home', now() - interval '3 hours');

prepare live_attendance(uuid) as
	select coalesce(string_agg(kind::text || ':' || coalesce(location, '-'), ' ' order by occurred_at, id), '')
	from public.attendance
	where member_id = $1 and deleted_at is null;

set local role authenticated;
select set_config('request.jwt.claim.sub', '48000000-0000-0000-0000-000000000001', true);

select is(
	public.attendance_add(null, 'clock_in', null, null, 'Office', null) #>> '{event,location}',
	'Office',
	'clocking in while working answers with the new clock-in'
);
select results_eq(
	$$execute live_attendance('48000000-0000-0000-0000-000000000011')$$,
	$$values ('clock_in:Home clock_out:- clock_in:Office')$$,
	'the shift that was open is clocked out before the new one starts'
);
select ok(
	(select max(occurred_at) filter (where kind = 'clock_out')
		< max(occurred_at) filter (where kind = 'clock_in')
	 from public.attendance
	 where member_id = '48000000-0000-0000-0000-000000000011' and deleted_at is null),
	'the clock-out sorts before the clock-in it makes way for'
);

select set_config('request.jwt.claim.sub', '48000000-0000-0000-0000-000000000002', true);
select lives_ok($block$do $$
declare
	written timestamptz := now() - interval '1 hour';
	answer jsonb;
begin
	answer := public.attendance_add(
		null,
		'clock_in',
		(written at time zone 'Asia/Seoul')::date,
		(written at time zone 'Asia/Seoul')::time,
		'Office',
		'forgot to clock in'
	);
	assert answer ->> 'status' = 'added', 'a record written by hand is added as written';
end $$;$block$, 'a clock-in written by hand is added');
select results_eq(
	$$execute live_attendance('48000000-0000-0000-0000-000000000012')$$,
	$$values ('clock_in:Home clock_in:Office')$$,
	'a record written by hand adds only what was written'
);

reset role;
select is(
	(select count(*)::integer from public.attendance
	 where member_id = '48000000-0000-0000-0000-000000000011' and kind = 'clock_out'),
	1,
	'exactly one clock-out is written for the move'
);

select * from finish();
rollback;
