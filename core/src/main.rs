// core/src/main.rs
use tonic::{transport::Server, Request, Response, Status};
use std::time::Instant;
use sqlx::postgres::PgPoolOptions;

pub mod game {
    tonic::include_proto!("game");
}

use game::game_service_server::{GameService, GameServiceServer};
use game::{ActionRequest, ActionResponse, TelemetryRequest, TelemetryResponse};

#[derive(Debug)]
pub struct MyGameService {
    boot_time: Instant,
    // Core database connection state pools matching global architecture requirements
    pg_pool: sqlx::PgPool,
    redis_client: redis::Client,
}

impl MyGameService {
    fn new(pg_pool: sqlx::PgPool, redis_client: redis::Client) -> Self {
        Self {
            boot_time: Instant::now(),
            pg_pool,
            redis_client,
        }
    }

    fn get_formatted_uptime(&self) -> String {
        let elapsed = self.boot_time.elapsed().as_secs();
        let days = elapsed / 86400;
        let hours = (elapsed % 86400) / 3600;
        let minutes = (elapsed % 3600) / 60;
        let seconds = elapsed % 60;

        format!("{}d {}h {}m {}s", days, hours, minutes, seconds)
    }

    async fn check_postgres(&self) -> bool {
        // sqlx has an internal non-blocking acquire check. If the pool is dead, it fails quickly.
        // We execute a light raw scalar ping check
        sqlx::query("SELECT 1")
            .execute(&self.pg_pool)
            .await
            .is_ok()
    }

    async fn check_redis(&self) -> bool {
        // Redis async connection can block if the host is down. 
        // To protect the runtime thread, we wrap it cleanly using spawn_blocking
        let client = self.redis_client.clone();
        
        let check_task = tokio::task::spawn_blocking(move || {
            // Use a short, strict connection timeout on the native TCP transport layer
            if let Ok(mut conn) = client.get_connection_with_timeout(std::time::Duration::from_millis(500)) {
                let p: redis::RedisResult<String> = redis::cmd("PING").query(&mut conn);
                p.is_ok()
            } else {
                false
            }
        });

        match tokio::time::timeout(std::time::Duration::from_millis(600), check_task).await {
            Ok(Ok(alive)) => alive,
            _ => false, // Safeguard block if the OS thread blocks
        }
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

    // Serve multi-plexed structural health metrics tree to Go Gateway proxy
    async fn get_server_telemetry(
        &self,
        _request: Request<TelemetryRequest>,
    ) -> Result<Response<TelemetryResponse>, Status> {
        let pg_alive = self.check_postgres().await;
        let redis_alive = self.check_redis().await;

        // Compile multi-plexed payload token string separated by pipe symbols
        let serialized_telemetry = format!(
            "{}|{}|{}",
            self.get_formatted_uptime(),
            pg_alive,
            redis_alive
        );

        let reply = TelemetryResponse {
            status: true,
            version: "v0.1.0-rust".to_string(),
            uptime: serialized_telemetry,
        };
        Ok(Response::new(reply))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("[Core] Connecting to relational storage engine...");
    
    // Internal secure cluster addresses matching docker-compose networking
    let database_url = "postgres://ogame_admin:ogame_secure_pass@postgres:5432/ogame_game_db";
    let redis_url = "redis://redis:6379/";

    // Establish dynamic connection pool for PostgreSQL
    let pg_pool = PgPoolOptions::new()
        .max_connections(5)
        .acquire_timeout(std::time::Duration::from_millis(500)) // Time out if pool cannot acquire
        .connect(database_url)
        .await?;
    println!("[Core] PostgreSQL connection channel successfully established.");

    // Establish dynamic asynchronous connection descriptor for Redis
    let redis_client = redis::Client::open(redis_url)?;
    println!("[Core] Redis cache bridge successfully initialized.");

    let addr = "0.0.0.0:50051".parse()?;
    let game_service = MyGameService::new(pg_pool, redis_client);

    println!("[Core] High-performance gRPC Core Service active on {}", addr);

    Server::builder()
        .add_service(GameServiceServer::new(game_service))
        .serve(addr)
        .await?;

    Ok(())
}