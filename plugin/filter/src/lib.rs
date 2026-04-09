

#[unsafe(no_mangle)]
fn get_secret_number() -> u32 {
    1116
}

// Allocate a fixed size of memory and return the raw pointer
#[unsafe(no_mangle)]
fn allocate_memory(size: usize) -> *mut u8 {
    let vec = Vec::with_capacity(size);

    let (ptr, _len, _cap) = vec.into_raw_parts();

    let ptr = ptr as *mut u8;
    ptr
}

// Main filter function
#[unsafe(no_mangle)]
fn process_request(ptr: *const u8, len: usize) -> u8 {
    let result_str = unsafe {std::slice::from_raw_parts(ptr, len)};
    let val = serde_json::from_str(ptr)
    if result_str == "suspicious" {1} else {0}
}

// Free memory
#[unsafe(no_mangle)]
fn free_memory(ptr : *mut u8, len: usize) {
    unsafe{ Vec::from_raw_parts(ptr, len, len);}
}