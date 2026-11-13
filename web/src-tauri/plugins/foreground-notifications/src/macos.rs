use std::sync::{Arc, Mutex, PoisonError};

use block2::{DynBlock, RcBlock};
use objc2::rc::Retained;
use objc2::runtime::{NSObject, NSObjectProtocol, ProtocolObject};
use objc2::{define_class, msg_send, sel, AnyThread, DefinedClass};
use objc2_user_notifications::{
    UNNotification, UNNotificationPresentationOptions, UNNotificationResponse,
    UNPushNotificationTrigger, UNUserNotificationCenter, UNUserNotificationCenterDelegate,
};
use tauri::{Manager, Runtime, State};

type WrappedDelegate = Retained<ProtocolObject<dyn UNUserNotificationCenterDelegate>>;

#[derive(Clone, Default)]
pub struct HeldBackTag(Arc<Mutex<String>>);

impl HeldBackTag {
    fn replace(&self, tag: String) {
        *self.0.lock().unwrap_or_else(PoisonError::into_inner) = tag;
    }

    fn holds_back(&self, thread: &str) -> bool {
        let held = self.0.lock().unwrap_or_else(PoisonError::into_inner);
        !held.is_empty() && *held == thread
    }
}

struct Presenter {
    wrapped: Option<WrappedDelegate>,
    held_back: HeldBackTag,
}

define_class!(
    #[unsafe(super(NSObject))]
    #[name = "InternkimForegroundNotifications"]
    #[ivars = Presenter]
    struct ForegroundDelegate;

    unsafe impl NSObjectProtocol for ForegroundDelegate {}

    unsafe impl UNUserNotificationCenterDelegate for ForegroundDelegate {
        #[unsafe(method(userNotificationCenter:willPresentNotification:withCompletionHandler:))]
        fn will_present(
            &self,
            center: &UNUserNotificationCenter,
            notification: &UNNotification,
            completion_handler: &DynBlock<dyn Fn(UNNotificationPresentationOptions)>,
        ) {
            let presenter = self.ivars();
            let Some(shown) = presenter.presentation_of_a_push(notification) else {
                return presenter.ask_the_wrapped_delegate(
                    center,
                    notification,
                    completion_handler,
                );
            };
            let answer = completion_handler.copy();
            let answer_as_decided =
                RcBlock::new(move |_: UNNotificationPresentationOptions| answer.call((shown,)));
            presenter.ask_the_wrapped_delegate(center, notification, &answer_as_decided);
        }

        #[unsafe(method(userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:))]
        fn did_receive(
            &self,
            center: &UNUserNotificationCenter,
            response: &UNNotificationResponse,
            completion_handler: &DynBlock<dyn Fn()>,
        ) {
            let selector =
                sel!(userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:);
            match self
                .ivars()
                .wrapped
                .as_ref()
                .filter(|wrapped| wrapped.respondsToSelector(selector))
            {
                Some(wrapped) => wrapped
                    .userNotificationCenter_didReceiveNotificationResponse_withCompletionHandler(
                        center,
                        response,
                        completion_handler,
                    ),
                None => completion_handler.call(()),
            }
        }
    }
);

impl Presenter {
    fn presentation_of_a_push(
        &self,
        notification: &UNNotification,
    ) -> Option<UNNotificationPresentationOptions> {
        let request = notification.request();
        request
            .trigger()?
            .downcast_ref::<UNPushNotificationTrigger>()?;
        let thread = request.content().threadIdentifier().to_string();
        Some(presentation(self.held_back.holds_back(&thread)))
    }

    fn ask_the_wrapped_delegate(
        &self,
        center: &UNUserNotificationCenter,
        notification: &UNNotification,
        completion_handler: &DynBlock<dyn Fn(UNNotificationPresentationOptions)>,
    ) {
        let selector = sel!(userNotificationCenter:willPresentNotification:withCompletionHandler:);
        match self
            .wrapped
            .as_ref()
            .filter(|wrapped| wrapped.respondsToSelector(selector))
        {
            Some(wrapped) => wrapped
                .userNotificationCenter_willPresentNotification_withCompletionHandler(
                    center,
                    notification,
                    completion_handler,
                ),
            None => completion_handler.call((UNNotificationPresentationOptions::empty(),)),
        }
    }
}

fn presentation(held_back: bool) -> UNNotificationPresentationOptions {
    if held_back {
        return UNNotificationPresentationOptions::empty();
    }
    UNNotificationPresentationOptions::Badge
        | UNNotificationPresentationOptions::Sound
        | UNNotificationPresentationOptions::Banner
        | UNNotificationPresentationOptions::List
}

pub fn present_pushes_while_the_app_is_open<R: Runtime>(app: &tauri::AppHandle<R>) {
    let held_back = HeldBackTag::default();
    app.manage(held_back.clone());
    let center = UNUserNotificationCenter::currentNotificationCenter();
    let presenter = Presenter {
        wrapped: center.delegate(),
        held_back,
    };
    let delegate = ForegroundDelegate::alloc().set_ivars(presenter);
    let delegate: Retained<ForegroundDelegate> = unsafe { msg_send![super(delegate), init] };
    center.setDelegate(Some(ProtocolObject::from_ref(&*delegate)));
    std::mem::forget(delegate);
}

#[tauri::command]
pub fn hold_back(tag: String, held_back: State<'_, HeldBackTag>) {
    held_back.replace(tag);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_push_for_the_open_conversation_is_held_back_and_any_other_is_shown() {
        let held_back = HeldBackTag::default();
        assert!(!held_back.holds_back("message:conversation-sample"));
        assert!(!held_back.holds_back(""));
        held_back.replace("message:conversation-sample".into());
        assert!(held_back.holds_back("message:conversation-sample"));
        assert!(!held_back.holds_back("message:another-conversation"));
        assert!(presentation(true).is_empty());
        assert!(presentation(false).contains(UNNotificationPresentationOptions::Banner));
        held_back.replace(String::new());
        assert!(!held_back.holds_back(""));
    }
}
