fn main() {
    // Temporary boot message until we add gRPC and Tokio logic
    println!("Game Core service started successfully in Docker!");
    loop {
        std::thread::sleep(std::time::Duration::from_secs(3600));
    }
}
