mod address;
#[cfg(mobile)]
mod phone;

#[cfg(mobile)]
pub use phone::init;
