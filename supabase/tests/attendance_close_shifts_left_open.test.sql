begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
	('49000000-0000-0000-0000-000000000001', 'left-open@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	('49000000-0000-0000-0000-000000000000', 'Sample Left Open', 'sample-left-open', 'KR', 'ko', 'Asia/Seoul',
	 '[{"name":"Office"},{"name":"Home"}]', '[[[],[],[],[],[],null,null]]', 480);

insert into public.member (id, company_id, email, user_id, status) values
	('49000000-0000-0000-0000-000000000011', '49000000-0000-0000-0000-000000000000',
	 'left-open@example.test', '49000000-0000-0000-0000-000000000001', 'active');

insert into public.attendance (member_id, kind, location, occurred_at) values
	('49000000-0000-0000-0000-000000000011', 'clock_in', 'Home', '2026-09-01 09:00+09'),
	('49000000-0000-0000-0000-000000000011', 'clock_in', 'Office', '2026-09-01 13:00+09'),
	('49000000-0000-0000-0000-000000000011', 'clock_out', null, '2026-09-01 18:00+09'),
	('49000000-0000-0000-0000-000000000011', 'clock_in', 'Home', '2026-09-03 09:00+09'),
	('49000000-0000-0000-0000-000000000011', 'clock_in', 'Office', '2026-09-05 09:00+09'),
	('49000000-0000-0000-0000-000000000011', 'clock_out', null, '2026-09-05 18:00+09');

prepare live_attendance as
	select string_agg(kind::text || '@' || to_char(occurred_at at time zone 'Asia/Seoul', 'MM-DD HH24:MI:SS.MS'), ' ' order by occurred_at, id)
	from public.attendance
	where member_id = '49000000-0000-0000-0000-000000000011' and deleted_at is null;

select is(
	internal.attendance_close_shifts_left_open(),
	1,
	'only the clock-in a later clock-in followed within a day is closed'
);
select results_eq(
	'execute live_attendance',
	$$values ('clock_in@09-01 09:00:00.000 clock_out@09-01 12:59:59.999 clock_in@09-01 13:00:00.000 clock_out@09-01 18:00:00.000 clock_in@09-03 09:00:00.000 clock_in@09-05 09:00:00.000 clock_out@09-05 18:00:00.000')$$,
	'the clock-out lands a millisecond before the clock-in that left the shift open'
);
select is(
	internal.attendance_close_shifts_left_open(),
	0,
	'running it again closes nothing more'
);
select is(
	(select edit_reason from public.attendance
	 where member_id = '49000000-0000-0000-0000-000000000011' and occurred_at = '2026-09-01 12:59:59.999+09'),
	null,
	'the added clock-out carries no hand-written reason'
);

select * from finish();
rollback;
