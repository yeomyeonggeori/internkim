-- 중단 is work stopped, not work cancelled; the enum takes the spoken word the
-- way planned and completed already did. Rows follow the rename on their own,
-- and no function names the old word.
alter type public.task_status rename value 'cancelled' to 'stopped';
