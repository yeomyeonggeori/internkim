use tauri::Url;

const FETCHABLE_SCHEMES: [&str; 2] = ["https", "http"];

pub fn fetchable_address(address: &str) -> Result<String, String> {
    let parsed = Url::parse(address).map_err(|error| format!("{address:?} is not an address to download from: {error}"))?;
    if !FETCHABLE_SCHEMES.contains(&parsed.scheme()) {
        return Err(format!("only http and https addresses are downloaded, not {}", parsed.scheme()));
    }
    Ok(parsed.into())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn takes_a_signed_store_address_as_it_is() {
        let signed = "https://store.example.com/object/sign/asset/%ED%9A%8C%EC%9D%98.pdf?token=sample&download=%ED%9A%8C%EC%9D%98.pdf";
        assert_eq!(fetchable_address(signed).unwrap(), signed);
    }

    #[test]
    fn refuses_what_the_phone_cannot_fetch_from_an_address() {
        assert!(fetchable_address("blob:https://intern.kim/6f1c2b3a").is_err());
        assert!(fetchable_address("file:///etc/hosts").is_err());
        assert!(fetchable_address("/files/api/download?path=a").is_err());
    }
}
