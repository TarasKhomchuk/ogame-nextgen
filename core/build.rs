// core/build.rs
// This script automatically compiles the protocol buffer definitions on build

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Using the correct method name '.compile' as required by tonic_build::configure()
    tonic_build::configure()
        .compile(
            &["/proto/game.proto"], 
            &["/proto"]
        )?;
    Ok(())
}
