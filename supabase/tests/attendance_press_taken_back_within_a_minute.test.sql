begin;
create extension if not exists pgtap with schema extensions;
select plan(12);

insert into auth.users (id, email) values
	('47000000-0000-0000-0000-000000000001', 'press-resumer@example.test'),
	('47000000-0000-0000-0000-000000000002', 'press-writer@example.test'),
	('47000000-0000-0000-0000-000000000003', 'press-returner@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	('47000000-0000-0000-0000-000000000000', 'Sample Press', 'sample-press', 'KR', 'ko', 'Asia/Seoul',
	 '[{"name":"Office"},{"name":"Branch"}]', '[[[],[],[],[],[],null,null]]', 480);

insert into public.member (id, company_id, email, user_id, status) values
	('47000000-0000-0000-0000-000000000011', '47000000-0000-0000-0000-000000000000',
	 'press-resumer@example.test', '47000000-0000-0000-0000-000000000001', 'active'),
	('47000000-0000-0000-0000-000000000012', '47000000-0000-0000-0000-000000000000',
	 'press-writer@example.test', '47000000-0000-0000-0000-000000000002', 'active'),
	('47000000-0000-0000-0000-000000000013', '47000000-0000-0000-0000-000000000000',
	 'press-returner@example.test', '47000000-0000-0000-0000-000000000003', 'active');

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	('47000000-0000-0000-0000-000000000101', '47000000-0000-0000-0000-000000000011',
	 'clock_in', 'Office', now() - interval '8 hours'),
	('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-000000000011',
	 'clock_out', null, now() - interval '10 seconds'),
	('47000000-0000-0000-0000-000000000201', '47000000-0000-0000-0000-000000000012',
	 'clock_in', 'Office', now() - interval '2 hours'),
	('47000000-0000-0000-0000-000000000202', '47000000-0000-0000-0000-000000000012',
	 'clock_out', null, now() - interval '90 seconds'),
	('47000000-0000-0000-0000-000000000301', '47000000-0000-0000-0000-000000000013',
	 'clock_in', 'Office', now() - interval '2 hours'),
	('47000000-0000-0000-0000-000000000302', '47000000-0000-0000-0000-000000000013',
	 'clock_out', null, now() - interval '5 minutes');

prepare live_attendance(uuid) as
	select coalesce(string_agg(kind::text || ':' || coalesce(location, '-'), ' ' order by occurred_at, id), '')
	from public.attendance
	where member_id = $1 and deleted_at is null;

set local role authenticated;
select set_config('request.jwt.claim.sub', '47000000-0000-0000-0000-000000000001', true);

select is(
	public.attendance_add(null, 'clock_in', null, null, 'Office', null) - 'backdated',
	jsonb_build_object('status', 'removed', 'eventID', '47000000-0000-0000-0000-000000000102'),
	'clocking back in within a minute takes the clock-out back'
);
select results_eq(
	$$execute live_attendance('47000000-0000-0000-0000-000000000011')$$,
	$$values ('clock_in:Office')$$,
	'the shift carries on as one'
);
select lives_ok($block$do $$
declare answer jsonb;
begin
	answer := public.attendance_add(null, 'clock_out', null, null, null, null);
	assert answer ->> 'status' = 'added', 'a clock-out long after the clock-in is recorded';
	answer := public.attendance_add(null, 'clock_in', null, null, null, null);
	assert answer ->> 'status' = 'removed', 'a default location resumes the shift it names';
	answer := public.attendance_add(null, 'clock_out', null, null, null, null);
	assert answer ->> 'status' = 'added', 'the shift closes again';
	answer := public.attendance_add(null, 'clock_in', null, null, 'Branch', null);
	assert answer ->> 'status' = 'added', 'clocking in somewhere else starts a new shift';
end $$;$block$, 'pressing back and forth keeps one shift until the place changes');
select results_eq(
	$$execute live_attendance('47000000-0000-0000-0000-000000000011')$$,
	$$values ('clock_in:Office clock_out:- clock_in:Branch')$$,
	'the two places stay two shifts'
);

select is(
	public.attendance_add(null, 'clock_out', null, null, null, null) ->> 'status',
	'removed',
	'clocking out within a minute takes the clock-in back'
);
select results_eq(
	$$execute live_attendance('47000000-0000-0000-0000-000000000011')$$,
	$$values ('clock_in:Office clock_out:-')$$,
	'no shift of a few seconds is left behind'
);

select lives_ok($block$do $$
declare answer jsonb;
begin
	answer := public.attendance_add(null, 'clock_in', null, null, 'Office', null);
	assert answer ->> 'status' = 'removed', 'the shift is resumed';
	answer := public.attendance_add(null, 'clock_in', null, null, 'Branch', null);
	assert answer ->> 'status' = 'added', 'moving to another place is recorded';
	answer := public.attendance_add(null, 'clock_out', null, null, null, null);
	assert answer ->> 'status' = 'removed', 'clocking out right after a move takes the move back';
end $$;$block$, 'a clock-out right after a move ends the day where it was');
select results_eq(
	$$execute live_attendance('47000000-0000-0000-0000-000000000011')$$,
	$$values ('clock_in:Office clock_out:-')$$,
	'the day ends at the move, with no shift of a few seconds left behind'
);

select set_config('request.jwt.claim.sub', '47000000-0000-0000-0000-000000000002', true);
select lives_ok($block$do $$
declare
	written timestamptz := now() - interval '45 seconds';
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
end $$;$block$, 'a record written by hand takes nothing back');
select results_eq(
	$$execute live_attendance('47000000-0000-0000-0000-000000000012')$$,
	$$values ('clock_in:Office clock_out:- clock_in:Office')$$,
	'the clock-out before the hand-written record is kept'
);

select set_config('request.jwt.claim.sub', '47000000-0000-0000-0000-000000000003', true);
select is(
	public.attendance_add(null, 'clock_in', null, null, 'Office', null) ->> 'status',
	'added',
	'clocking in more than a minute after the clock-out starts a new shift'
);

reset role;
select is(
	(select edit_reason from public.attendance where id = '47000000-0000-0000-0000-000000000102'),
	'reversed within a minute',
	'the record taken back says why'
);

select * from finish();
rollback;
