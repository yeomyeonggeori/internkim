alter policy box_backup_read_by_the_company_boxes on storage.objects
  using (
    bucket_id = 'box_backup'
    and name collate "C" >= (select public.my_app_company())::text || '/'
    and name collate "C" < (select public.my_app_company())::text || '0'
  );

alter policy box_backup_written_by_the_serving_box on storage.objects
  with check (
    bucket_id = 'box_backup'
    and (select public.my_app_box_role()) = 'service'
    and name collate "C" >= (select public.my_app_company())::text || '/'
    and name collate "C" < (select public.my_app_company())::text || '0'
  );

alter policy box_backup_replaced_by_the_serving_box on storage.objects
  using (
    bucket_id = 'box_backup'
    and (select public.my_app_box_role()) = 'service'
    and name collate "C" >= (select public.my_app_company())::text || '/'
    and name collate "C" < (select public.my_app_company())::text || '0'
  )
  with check (
    bucket_id = 'box_backup'
    and (select public.my_app_box_role()) = 'service'
    and name collate "C" >= (select public.my_app_company())::text || '/'
    and name collate "C" < (select public.my_app_company())::text || '0'
  );

alter policy box_backup_pruned_by_the_serving_box on storage.objects
  using (
    bucket_id = 'box_backup'
    and (select public.my_app_box_role()) = 'service'
    and name collate "C" >= (select public.my_app_company())::text || '/'
    and name collate "C" < (select public.my_app_company())::text || '0'
  );
