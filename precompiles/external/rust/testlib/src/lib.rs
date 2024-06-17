use std::ffi::CStr;

#[no_mangle]
pub unsafe extern "C" fn test_func(input: *const libc::c_char) -> *const libc::c_char {
    // Convert the input from a C string to a Rust string
    let input_cstr = unsafe { CStr::from_ptr(input) };
    let input = input_cstr.to_str().unwrap().to_string();

    // You should do some validation here

    // Call the Rust function
    let output = format!("my name is {}", input);

    // Convert the output from a Rust string to a C string
    let output_cstr = match std::ffi::CString::new(output) {
        Ok(cstring) => cstring,
        Err(e) => {
            println!("({})", e);
            return ::std::ptr::null();
        }
    };

    output_cstr.into_raw()
}