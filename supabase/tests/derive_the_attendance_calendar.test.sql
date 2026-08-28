begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('92000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null);

-- The policy every fixture starts from: weekdays, eight hours, one hour of lunch.
create or replace function pg_temp.weekday_policy() returns jsonb language sql immutable as $$
  select jsonb_build_object(
    'workMode', 'autonomous',
    'workingWeekdays', jsonb_build_array(1, 2, 3, 4, 5),
    'dailyTargetMinutes', 480,
    'weeklyTargetMinutes', 0,
    'referenceStartTime', '09:00',
    'fixedStartTime', '',
    'fixedEndTime', '',
    'coreTimeEnabled', false,
    'coreStartTime', '',
    'coreEndTime', '',
    'breakPeriods', jsonb_build_array(jsonb_build_object('startTime', '12:00', 'endTime', '13:00')),
    'nightStartTime', '22:00',
    'nightEndTime', '06:00'
  );
$$;

-- What the device pushed: a row a day, the mode repeated, weekends not worked.
create or replace function pg_temp.projected_days(
  first_day date,
  last_day date,
  work_mode text,
  holidays date[]
) returns jsonb language sql immutable as $$
  select jsonb_agg(
    jsonb_build_object(
      'date', to_char(day, 'YYYY-MM-DD'),
      'workMode', work_mode,
      'workingDate', extract(isodow from day) between 1 and 5 and not (day = any (holidays)),
      'holiday', day = any (holidays)
    )
    order by day
  )
  from generate_series(first_day, last_day, interval '1 day') as generated(day);
$$;

select lives_ok($block$do $$
declare
  calendar jsonb := pg_temp.projected_days(
    date '2026-07-01', date '2026-08-31', 'autonomous',
    array[date '2026-07-17', date '2026-08-17']
  );
  derived jsonb;
begin
  assert jsonb_array_length(calendar) = 62, 'the fixture is the 62 rows the issue describes';

  derived := internal.attendance_policy_from_calendar(calendar, pg_temp.weekday_policy());

  assert jsonb_array_length(derived -> 'revisions') = 1,
    'a company that never changed its policy ends on one revision';
  assert derived -> 'revisions' -> 0 ->> 'effectiveDate' = '1970-01-01',
    'the only revision answers for every date before it';
  assert derived -> 'revisions' -> 0 ->> 'workMode' = 'autonomous',
    'the mode the calendar recorded is the mode the revision carries';
  assert derived -> 'revisions' -> 0 -> 'workingWeekdays' = '[1, 2, 3, 4, 5]'::jsonb,
    'the weekdays actually worked become the working weekdays';
  assert internal.attendance_calendar_disagrees_on(calendar, derived) is null,
    'every date is judged as it was';
end $$;$block$, '62 dated rows become one revision and judge every date the same');

select lives_ok($block$do $$
declare
  calendar jsonb;
  derived jsonb;
begin
  calendar := pg_temp.projected_days(date '2026-07-01', date '2026-07-31', 'flexible', array[]::date[])
    || pg_temp.projected_days(date '2026-08-01', date '2026-08-31', 'autonomous', array[]::date[]);

  derived := internal.attendance_policy_from_calendar(calendar, pg_temp.weekday_policy());

  assert jsonb_array_length(derived -> 'revisions') = 2,
    'a mode change is a second revision';
  assert derived -> 'revisions' -> 0 ->> 'effectiveDate' = '1970-01-01'
     and derived -> 'revisions' -> 0 ->> 'workMode' = 'flexible',
    'the earlier mode covers everything before the change';
  assert derived -> 'revisions' -> 1 ->> 'effectiveDate' = '2026-08-01'
     and derived -> 'revisions' -> 1 ->> 'workMode' = 'autonomous',
    'the later mode takes effect the day the calendar first recorded it';
  assert internal.attendance_calendar_disagrees_on(calendar, derived) is null,
    'both periods keep the judgement they had';
end $$;$block$, 'a policy change becomes two revisions and neither period moves');

select lives_ok($block$do $$
declare
  calendar jsonb;
  derived jsonb;
begin
  calendar := pg_temp.projected_days(date '2026-08-01', date '2026-08-31', 'autonomous', array[]::date[]);
  derived := internal.attendance_policy_from_calendar(calendar, pg_temp.weekday_policy());

  perform public.attendance_policy_save('92000000-0000-0000-0000-0000000000c1', derived);

  assert (
    select rules -> 'attendanceWorkPolicy' -> 'revisions' -> 0 ->> 'workMode'
    from public.company where id = '92000000-0000-0000-0000-0000000000c1'
  ) = 'autonomous', 'a derived policy is a policy the saver accepts';
  assert (
    select rules ? 'attendanceCalendar'
    from public.company where id = '92000000-0000-0000-0000-0000000000c1'
  ) = false, 'nothing wrote a dated row back';
end $$;$block$, 'a derived policy saves and leaves no dated rows behind');

select lives_ok($block$do $$
declare
  flat jsonb := pg_temp.weekday_policy();
  promoted jsonb;
begin
  promoted := internal.attendance_policy_with_revisions(flat);

  assert jsonb_array_length(promoted -> 'revisions') = 1,
    'a device still sending one flat policy is answered, not refused';
  assert promoted -> 'revisions' -> 0 ->> 'effectiveDate' = '1970-01-01',
    'a flat policy is the policy that has always been in force';
  assert promoted -> 'revisions' -> 0 ->> 'workMode' = flat ->> 'workMode',
    'promotion changes nothing but the effective date';

  assert internal.attendance_policy_with_revisions(promoted) = promoted,
    'a policy that already speaks revisions is left alone';
end $$;$block$, 'a flat policy is promoted to the revision that always applied');

select lives_ok($block$do $$
declare
  unordered jsonb;
  normalized jsonb;
begin
  unordered := jsonb_build_object('version', 1, 'revisions', jsonb_build_array(
    pg_temp.weekday_policy() || jsonb_build_object('effectiveDate', '2026-08-01'),
    pg_temp.weekday_policy() || jsonb_build_object('effectiveDate', '2026-03-01')
  ));

  normalized := internal.attendance_policy_normalize(unordered);

  assert normalized -> 'revisions' -> 0 ->> 'effectiveDate' = '1970-01-01',
    'the earliest revision is stamped at the epoch whatever it arrived as';
  assert normalized -> 'revisions' -> 1 ->> 'effectiveDate' = '2026-08-01',
    'the rest keep their own dates, in order';
end $$;$block$, 'revisions are stored in effective-date order');

select throws_ok(
  $$select internal.attendance_policy_normalize(
      jsonb_build_object('version', 1, 'revisions', jsonb_build_array(
        pg_temp.weekday_policy() || jsonb_build_object('effectiveDate', '2026-08-01'),
        pg_temp.weekday_policy() || jsonb_build_object('effectiveDate', '2026-08-01')
      ))
    )$$,
  'two attendance work policy revisions take effect on one date'
);

select throws_ok(
  $$select internal.attendance_policy_normalize(
      jsonb_build_object('version', 1, 'revisions', '[]'::jsonb)
    )$$,
  'an attendance work policy is version 1 and a non-empty list of revisions'
);

-- A calendar that recorded a mode the snapshot cannot describe is reported, not
-- guessed at: a fixed schedule needs the hours it fixed, and no dated row has them.
select throws_ok(
  $$select internal.attendance_policy_from_calendar(
      pg_temp.projected_days(date '2026-08-03', date '2026-08-07', 'fixed', array[]::date[]),
      pg_temp.weekday_policy()
    )$$,
  '23514',
  'the calendar says fixed mode from 2026-08-03, and it cannot supply what that mode needs: attendance fixed work policy is invalid',
  'a mode the calendar cannot describe is named rather than guessed at'
);

select is(
  internal.attendance_calendar_disagrees_on(
    jsonb_build_array(jsonb_build_object(
      'date', '2026-08-08', 'workMode', 'autonomous', 'workingDate', true, 'holiday', false
    )),
    internal.attendance_policy_normalize(
      internal.attendance_policy_with_revisions(pg_temp.weekday_policy())
    )
  ),
  '2026-08-08',
  'a date the revisions would judge differently is named'
);

select * from finish();
rollback;
