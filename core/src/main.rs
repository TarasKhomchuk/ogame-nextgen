// core/src/main.rs
use tonic::{transport::Server, Request, Response, Status};
use std::time::Instant;
use std::sync::Arc;
use tokio::sync::RwLock;
use sqlx::postgres::PgPoolOptions;
use sqlx::Row;

pub mod game {
    tonic::include_proto!("game");
}

use game::game_service_server::{GameService, GameServiceServer};
use game::{
    ActionRequest, ActionResponse, TelemetryRequest, TelemetryResponse,
    DatabaseCatalogRequest, DatabaseCatalogResponse, DatabaseDetails,
    SwitchDatabaseRequest, SwitchDatabaseResponse, InitSchemaRequest, InitSchemaResponse
};

#[derive(Debug)]
pub struct MyGameService {
    boot_time: Instant,
    redis_client: redis::Client,
    // Permanent link to system catalog instance for administrative operations only
    system_pg_pool: sqlx::PgPool,
    // Optional dynamic pool container matching explicit admin choice (None = Server Disconnected)
    active_pg_pool: Arc<RwLock<Option<sqlx::PgPool>>>,
    // Optional tracking string for active database name token configuration
    active_db_name: Arc<RwLock<Option<String>>>,
}

impl MyGameService {
    fn new(system_pg_pool: sqlx::PgPool, redis_client: redis::Client) -> Self {
        Self {
            boot_time: Instant::now(),
            redis_client,
            system_pg_pool,
            active_pg_pool: Arc::new(RwLock::new(None)), // Starts securely disconnected
            active_db_name: Arc::new(RwLock::new(None)), // No default database assigned
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
        // Read lock and evaluate if an active pool was explicitly mounted by admin
        let pool_guard = self.active_pg_pool.read().await;
        if let Some(ref pool) = *pool_guard {
            sqlx::query("SELECT 1").execute(pool).await.is_ok()
        } else {
            false // Firm offline state since no database is bound to the engine
        }
    }

    async fn check_redis(&self) -> bool {
        let client = self.redis_client.clone();
        let check_task = tokio::task::spawn_blocking(move || {
            if let Ok(mut conn) = client.get_connection_with_timeout(std::time::Duration::from_millis(500)) {
                let p: redis::RedisResult<String> = redis::cmd("PING").query(&mut conn);
                p.is_ok()
            } else {
                false
            }
        });
        match tokio::time::timeout(std::time::Duration::from_millis(600), check_task).await {
            Ok(Ok(alive)) => alive,
            _ => false,
        }
    }

    async fn init_database_schema(
        &self,
        request: Request<InitSchemaRequest>,
    ) -> Result<Response<InitSchemaResponse>, Status> {
        let req = request.into_inner();
        let name_guard = self.active_db_name.read().await;
        
        let active_name = match &*name_guard {
            Some(name) => name,
            None => return Ok(Response::new(InitSchemaResponse {
                success: false,
                message: "Command rejected: Cannot initialize schema because no active database is connected.".to_string(),
            })),
        };

        // 1. Read the defined DDL tables architecture from disk path
        let sql_script = match std::fs::read_to_string("migrations/0001_init_game_schema.sql") {
            Ok(content) => content,
            Err(e) => return Ok(Response::new(InitSchemaResponse {
                success: false,
                message: format!("Failed to read schema blueprint file from disk: {}", e),
            })),
        };

        let pool_guard = self.active_pg_pool.read().await;
        let pool = pool_guard.as_ref().unwrap();

        // 2. Execute SQL tables deployment transaction block
        if let Err(e) = sqlx::raw_sql(&sql_script).execute(pool).await {
            return Ok(Response::new(InitSchemaResponse {
                success: false,
                message: format!("DDL execution failure during transaction parsing sequence: {}", e),
            }));
        }

        // 3. Cryptographically hash the administrator password vector using bcrypt (cost factor = 10)
        let hashed_password = match bcrypt::hash(&req.admin_password, 10) {
            Ok(h) => h,
            Err(e) => return Ok(Response::new(InitSchemaResponse {
                success: false,
                message: format!("Internal security error during credentials hashing sequence: {}", e),
            })),
        };

        // 4. Securely inject the Provisioned Master Administrator into the clean 'users' table
        let insert_result = sqlx::query(
            "INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'SUPERADMIN') ON CONFLICT (username) DO NOTHING"
        )
        .bind(&req.admin_username)
        .bind(&hashed_password)
        .execute(pool)
        .await;

        match insert_result {
            Ok(_) => {
                println!("[Core] Success: Master administrator account '{}' provisioned into users system.", req.admin_username);
                Ok(Response::new(InitSchemaResponse {
                    success: true,
                    message: format!(
                        "Database tables initialized and master account '{}' successfully provisioned on context '{}'!", 
                        req.admin_username, active_name
                    ),
                }))
            },
            Err(e) => Ok(Response::new(InitSchemaResponse {
                success: false,
                message: format!("Schema built successfully, but master account injection failed: {}", e),
            })),
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
        
        // Block actions immediately if administrator hasn't mounted an active database backend yet
        let name_guard = self.active_db_name.read().await;
        if name_guard.is_none() {
            return Err(Status::failed_precondition("Game engine is offline: No database connected by administrator."));
        }

        println!("[Core] Processing game action: '{}'", req.action);
        let reply = ActionResponse {
            status: "processed".to_string(),
            msg_id: req.msg_id,
            payload: r#"{"message": "Action completed successfully!"}"#.to_string(),
        };
        Ok(Response::new(reply))
    }

    async fn get_server_telemetry(
        &self,
        _request: Request<TelemetryRequest>,
    ) -> Result<Response<TelemetryResponse>, Status> {
        // 1. Check if the physical PostgreSQL container is alive via system pool metadata check
        let pg_server_alive = sqlx::query("SELECT 1").execute(&self.system_pg_pool).await.is_ok();
        let redis_alive = self.check_redis().await;
        
        let db_label = {
            let name_guard = self.active_db_name.read().await;
            name_guard.clone().unwrap_or_else(|| "DISCONNECTED".to_string())
        };

        // Serialized token mapping format layout: "uptime|pg_server_alive|redis_alive|active_db_label"
        let serialized_telemetry = format!(
            "{}|{}|{}|{}", 
            self.get_formatted_uptime(), 
            pg_server_alive, 
            redis_alive, 
            db_label
        );

        let reply = TelemetryResponse {
            status: true,
            version: "v0.1.0-rust".to_string(),
            uptime: serialized_telemetry,
        };
        Ok(Response::new(reply))
    }

    async fn get_database_catalog(
        &self,
        _request: Request<DatabaseCatalogRequest>,
    ) -> Result<Response<DatabaseCatalogResponse>, Status> {
        // Enumerate clean live relational databases excluding template layers
        let rows = sqlx::query("SELECT datname FROM pg_database WHERE datistemplate = false AND datname != 'postgres'")
            .fetch_all(&self.system_pg_pool)
            .await
            .map_err(|e| Status::internal(e.to_string()))?;

        let current_active = self.active_db_name.read().await;
        let mut db_list = Vec::new();

        for row in rows {
            let name: String = row.get("datname");
            let is_active = match &*current_active {
                Some(active_name) => *active_name == name,
                None => false,
            };
            db_list.push(DatabaseDetails { name, is_active });
        }

        Ok(Response::new(DatabaseCatalogResponse { databases: db_list }))
    }

    async fn switch_or_create_database(
        &self,
        request: Request<SwitchDatabaseRequest>,
    ) -> Result<Response<SwitchDatabaseResponse>, Status> {
        let target_db = request.into_inner().database_name;

        // An empty target means the administrator explicitly requested to DISCONNECT all active databases
        if target_db.is_empty() {
            let mut active_pool_guard = self.active_pg_pool.write().await;
            let mut active_name_guard = self.active_db_name.write().await;
            
            *active_pool_guard = None;
            *active_name_guard = None;
            
            println!("[Core] Administrator command completed: All databases decoupled from the server.");
            return Ok(Response::new(SwitchDatabaseResponse {
                success: true,
                message: "Database cluster detached successfully. Game engine is now offline.".to_string(),
            }));
        }

        if target_db.chars().any(|c| !c.is_alphanumeric() && c != '_') {
            return Ok(Response::new(SwitchDatabaseResponse {
                success: false,
                message: "Security rejected: Invalid characters in database name allocation.".to_string(),
            }));
        }

        // 1. Verify existence: if it doesn't exist, create it cleanly upon specific explicit intent
        let check_query = format!("SELECT 1 FROM pg_database WHERE datname = '{}'", target_db);
        let exists = sqlx::query(&check_query).fetch_optional(&self.system_pg_pool).await
            .map_err(|e| Status::internal(e.to_string()))?.is_some();

        if !exists {
            let create_query = format!("CREATE DATABASE {}", target_db);
            sqlx::query(&create_query).execute(&self.system_pg_pool).await
                .map_err(|e| Status::internal(e.to_string()))?;
            println!("[Core] Explicit Admin Instruction: Provisioned new database '{}'", target_db);
        }

        // 2. Connect to the chosen database requested by admin configuration profile
        let target_url = format!("postgres://ogame_admin:ogame_secure_pass@postgres:5432/{}", target_db);
        let new_pool = PgPoolOptions::new()
            .max_connections(5)
            .acquire_timeout(std::time::Duration::from_millis(500))
            .connect(&target_url)
            .await
            .map_err(|e| Status::internal(format!("Failed to connect to requested database: {}", e)))?;

        // 3. Mount the initialized pool channel to active cluster runtime thread context
        let mut active_pool_guard = self.active_pg_pool.write().await;
        let mut active_name_guard = self.active_db_name.write().await;
        
        *active_pool_guard = Some(new_pool);
        *active_name_guard = Some(target_db.clone());

        println!("[Core] Success: Active database context context-swapped to '{}'", target_db);
        Ok(Response::new(SwitchDatabaseResponse {
            success: true,
            message: format!("Database connection successfully bound to pool context '{}'", target_db),
        }))
    }

    async fn init_database_schema(
        &self,
        _request: Request<InitSchemaRequest>,
    ) -> Result<Response<InitSchemaResponse>, Status> {
        let name_guard = self.active_db_name.read().await;
        let active_name = match &*name_guard {
            Some(name) => name,
            None => return Ok(Response::new(InitSchemaResponse {
                success: false,
                message: "Command rejected: Cannot initialize schema because no active database is connected.".to_string(),
            })),
        };

        let sql_script = match std::fs::read_to_string("migrations/0001_init_game_schema.sql") {
            Ok(content) => content,
            Err(e) => return Ok(Response::new(InitSchemaResponse {
                success: false,
                message: format!("Failed to read schema blueprint file from disk: {}", e),
            })),
        };

        let pool_guard = self.active_pg_pool.read().await;
        let pool = pool_guard.as_ref().unwrap();

        match sqlx::raw_sql(&sql_script).execute(pool).await {
            Ok(_) => {
                Ok(Response::new(InitSchemaResponse {
                    success: true,
                    message: format!("Structural game tables migration deployed successfully onto target context '{}'", active_name),
                }))
            },
            Err(e) => Ok(Response::new(InitSchemaResponse {
                success: false,
                message: format!("DDL execution failure during transaction parsing sequence: {}", e),
            })),
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("[Core] Standardizing multi-database runtime clusters engine...");
    
    // System database metadata connection URL
    let system_url = "postgres://ogame_admin:ogame_secure_pass@postgres:5432/postgres";
    let redis_url = "redis://redis:6379/";

    // 1. Establish administrative master catalog connections link channel only
    let system_pg_pool = PgPoolOptions::new()
        .max_connections(2)
        .acquire_timeout(std::time::Duration::from_millis(500))
        .connect(system_url)
        .await?;
    println!("[Core] PostgreSQL administrative context initialized successfully.");

    let redis_client = redis::Client::open(redis_url)?;
    let addr = "0.0.0.0:50051".parse()?;

    // Instantiate game service with completely disconnected dynamic states containers
    let game_service = MyGameService::new(system_pg_pool, redis_client);

    println!("[Core] High-performance Isolated Cluster Kernel serving on {}", addr);

    Server::builder()
        .add_service(GameServiceServer::new(game_service))
        .serve(addr).await?;
        
    Ok(())
}