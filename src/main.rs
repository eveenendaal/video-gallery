mod config;
mod handlers;
mod models;
mod services;
mod utils;

use crate::config::Config;
use crate::handlers::admin_handlers::*;
use crate::handlers::handlers::*;
use crate::handlers::AppState;
use crate::services::GalleryService;
use axum::{
    routing::{get, post},
    Router,
};
use clap::Parser;
use std::sync::Arc;
use tera::Tera;
use tower_http::services::ServeDir;
use tower_http::trace::TraceLayer;
use tracing::info;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

const VERSION: &str = env!("CARGO_PKG_VERSION");

#[derive(Parser)]
#[command(name = "video-gallery")]
#[command(version = VERSION)]
#[command(about = "Video Gallery - web server for video galleries", long_about = None)]
struct Cli {
    /// Set the SECRET_KEY (overrides environment variable)
    #[arg(short, long)]
    secret_key: Option<String>,

    /// Set the BUCKET_NAME (overrides environment variable)
    #[arg(short, long)]
    bucket: Option<String>,

    /// Set the PORT (overrides environment variable)
    #[arg(short, long)]
    port: Option<String>,
}

#[tokio::main]
async fn main() {
    // Initialize tracing
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| format!("{}=info,tower_http=info", env!("CARGO_PKG_NAME").replace("-", "_")).into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    let cli = Cli::parse();

    // Override environment variables with CLI args if provided
    if let Some(secret_key) = cli.secret_key {
        std::env::set_var("SECRET_KEY", secret_key);
    }
    if let Some(bucket) = cli.bucket {
        std::env::set_var("BUCKET_NAME", bucket);
    }
    if let Some(port) = cli.port {
        std::env::set_var("PORT", port);
    }

    // Load configuration
    let config = match Config::load() {
        Ok(cfg) => Arc::new(cfg),
        Err(e) => {
            eprintln!("Failed to load configuration: {}", e);
            std::process::exit(1);
        }
    };

    serve_website(config).await;
}

async fn serve_website(config: Arc<Config>) {
    // Initialize services
    let gallery_service = GalleryService::new(config.clone());

    // Initialize Tera templates
    let mut tera = match Tera::new("assets/templates/**/*.html") {
        Ok(t) => t,
        Err(e) => {
            eprintln!("Template parsing error: {}", e);
            std::process::exit(1);
        }
    };
    tera.autoescape_on(vec![".html"]);

    // Create app state
    let state = Arc::new(AppState {
        config: config.clone(),
        gallery_service,
        tera,
    });

    // Build routes
    let app = Router::new()
        // Static files
        .nest_service("/", ServeDir::new("public"))
        // Gallery routes
        .route(
            &format!("/{}/index", config.secret_key),
            get(gallery_handler),
        )
        .route(&format!("/{}/feed", config.secret_key), get(feed_handler))
        .route("/gallery/:stub", get(page_handler))
        // Admin routes
        .route(&format!("/{}/admin", config.secret_key), get(admin_handler))
        .route(
            &format!("/{}/admin/api/generate-thumbnail", config.secret_key),
            get(generate_thumbnail_handler),
        )
        .route(
            &format!("/{}/admin/api/clear-thumbnail", config.secret_key),
            post(clear_thumbnail_handler),
        )
        .route(
            &format!("/{}/admin/api/bulk-generate-thumbnails", config.secret_key),
            post(bulk_generate_thumbnails_handler),
        )
        .route(
            &format!("/{}/admin/api/bulk-clear-thumbnails", config.secret_key),
            post(bulk_clear_thumbnails_handler),
        )
        .route(
            &format!("/{}/admin/api/fetch-movie-poster", config.secret_key),
            get(fetch_movie_poster_handler),
        )
        .route(
            &format!("/{}/admin/api/search-movie-poster", config.secret_key),
            get(search_movie_poster_handler),
        )
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    // Print startup message
    config.print_server_start_message();

    // Start server
    let listener = tokio::net::TcpListener::bind(&config.server_address())
        .await
        .unwrap();

    info!("Server listening on {}", config.server_address());

    axum::serve(listener, app).await.unwrap();
}

