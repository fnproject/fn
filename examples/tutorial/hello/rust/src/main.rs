use std::io;
use std::io::Read;

fn main() {
    let mut buffer = String::new();
    let stdin = io::stdin();
    if stdin.lock().read_to_string(&mut buffer).is_ok() {
        println!("Hello {}", buffer.trim());
    }
}
