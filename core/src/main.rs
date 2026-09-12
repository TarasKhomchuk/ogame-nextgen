// core/src/main.rs
use tonic::{transport::Server, Request, Response, Status};
use std::time::Instant;

pub mod game {
    tonic::include_proto!("game");
}

use game::game_service_server::{GameService, GameServiceServer};
use game::{ActionRequest, ActionResponse, TelemetryRequest, TelemetryResponse};

#[derive(Debug)]
pub struct MyGameService {
    // Keep track of the precise instant the microservice kernel booted up
    boot_time: Instant,
}

impl MyGameService {
    fn new() -> Self {
        Self {
            boot_time: Instant::now(),
        }
    }

    // Helper function to format duration into human-readable uptime string
    fn get_formatted_uptime(&self) -> String {
        let elapsed = self.boot_time.elapsed().as_secs();
        let days = elapsed / 86400;
        let hours = (elapsed % 86400) / 3600;
        let minutes = (elapsed % 3600) / 60;
        let seconds = elapsed % 60;

        format!("{}d {}h {}m {}s", days, hours, minutes, seconds)
    }
}

#[tonic::async_trait]
impl GameService for MyGameService {
    async fn process_action(
        &self,
        request: Request<ActionRequest>,
    ) -> Result<Response<ActionResponse>, Status> {
        let req = request.into_inner();
        println!("[Core] Action execution multiplexer invoked: '{}'", req.action);

        let reply = ActionResponse {
            status: "processed".to_string(),
            msg_id: req.msg_id,
            payload: r#"{"message": "Hello from Rust bare-metal Core!"}"#.to_string(),
        };
        Ok(Response::new(reply))
    }

    // New RPC implementation serving live internal telemetry pipelines
    async fn get_server_telemetry(
        &self,
        _request: Request<TelemetryRequest>,
    ) -> Result<Response<TelemetryResponse>, Status> {
        let reply = TelemetryResponse {
            status: true,
            version: "v0.1.0-rust".to_string(),
            uptime: self.get_formatted_uptime(),
        };
        Ok(Response::new(reply))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = "0.0.0.0:50051".parse()?;
    let game_service = MyGameService::new();

    println!("[Core] High-performance gRPC Core Service active on {}", addr);

    Server::builder()
        .add_service(GameServiceServer::new(game_service))
        .serve(addr)
        .await?;

    Ok(())
}
