alter table public.push_device drop constraint push_device_kind_check;

alter table public.push_device add constraint push_device_kind_check
  check (kind in ('web-push', 'apns', 'fcm', 'apns-activity-start', 'apns-activity'));
