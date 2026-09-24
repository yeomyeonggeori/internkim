begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
	('44000000-0000-0000-0000-000000000001', 'time-behind@example.test'),
	('44000000-0000-0000-0000-000000000002', 'time-ahead@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	('44000000-0000-0000-0000-000000000000', 'Sample Time', 'sample-time', 'KR', 'ko', 'Asia/Seoul',
	 '[{"name":"Office"}]', '[[[],[],[],[],[],null,null]]', 480);

insert into public.member (id, company_id, email, user_id, status) values
	('44000000-0000-0000-0000-000000000011', '44000000-0000-0000-0000-000000000000',
	 'time-behind@example.test', '44000000-0000-0000-0000-000000000001', 'active'),
	('44000000-0000-0000-0000-000000000012', '44000000-0000-0000-0000-000000000000',
	 'time-ahead@example.test', '44000000-0000-0000-0000-000000000002', 'active');

create function pg_temp.clock_in_at(clock_time text) returns timestamptz language plpgsql as $$
declare
	answer jsonb := public.attendance_add(null, 'clock_in', null, clock_time::time, null, null);
begin
	return (answer -> 'event' ->> 'occurredAt')::timestamptz;
end;
$$;

set local role authenticated;

select set_config('request.jwt.claim.sub', '44000000-0000-0000-0000-000000000001', true);
select is(
	pg_temp.clock_in_at(to_char((now() - interval '1 minute') at time zone 'Asia/Seoul', 'HH24:MI')),
	date_trunc('minute', now() - interval '1 minute'),
	'a time that has already come today is today'
);

select set_config('request.jwt.claim.sub', '44000000-0000-0000-0000-000000000002', true);
select is(
	pg_temp.clock_in_at(to_char((now() + interval '1 minute') at time zone 'Asia/Seoul', 'HH24:MI')),
	date_trunc('minute', now() + interval '1 minute') - interval '1 day',
	'a time still ahead today is yesterday'
);

select throws_ok(
	$$select public.attendance_add(null, 'clock_out', current_date, null, null, null)$$,
	'23514',
	'an attendance record written by hand for a given day names its time',
	'a day without a time is still refused'
);

select * from finish();
rollback;
