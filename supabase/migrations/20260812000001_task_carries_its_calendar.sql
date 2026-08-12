alter type public.task_status add value if not exists 'requested';
alter type public.task_status add value if not exists 'rejected';

alter table public.task add column calendar jsonb;
