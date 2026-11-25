use std::ptr::NonNull;

use block2::RcBlock;
use objc2_foundation::NSArray;
use objc2_user_notifications::{UNNotification, UNUserNotificationCenter};

#[tauri::command]
pub fn clear_delivered(tag: String) {
    if tag.is_empty() {
        return;
    }
    let remove_the_read_ones = RcBlock::new(move |delivered: NonNull<NSArray<UNNotification>>| {
        let delivered = unsafe { delivered.as_ref() };
        let identifiers: Vec<_> = delivered
            .iter()
            .filter(|notification| notification.request().content().threadIdentifier().to_string() == tag)
            .map(|notification| notification.request().identifier())
            .collect();
        if identifiers.is_empty() {
            return;
        }
        UNUserNotificationCenter::currentNotificationCenter()
            .removeDeliveredNotificationsWithIdentifiers(&NSArray::from_retained_slice(&identifiers));
    });
    UNUserNotificationCenter::currentNotificationCenter()
        .getDeliveredNotificationsWithCompletionHandler(&remove_the_read_ones);
}
