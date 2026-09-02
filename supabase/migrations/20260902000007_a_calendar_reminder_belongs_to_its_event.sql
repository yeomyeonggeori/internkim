select cron.unschedule('announce-the-day')
where exists (select 1 from cron.job where jobname = 'announce-the-day');

drop function if exists public.announce_the_day();

update public.member
set notification_settings = notification_settings - 'calendarAt'
where notification_settings ? 'calendarAt';
