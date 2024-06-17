mod utils;

use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub fn greet(input: u64) {
    alert("Hello, wasmlib!");
}
