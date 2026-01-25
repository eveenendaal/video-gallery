use crate::config::Config;
use crate::models::{Gallery, Index};
use crate::services::GalleryService;
use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::{Html, IntoResponse, Response},
    Json,
};
use std::sync::Arc;
use tera::{Context, Tera};
use tracing::{error, info};

pub struct AppState {
    pub config: Arc<Config>,
    pub gallery_service: GalleryService,
    pub tera: Tera,
}

pub async fn gallery_handler(
    State(state): State<Arc<AppState>>,
) -> Result<Response, StatusCode> {
    info!("Generating Index");

    let categories = state.gallery_service.get_categories().await;

    let mut context = Context::new();
    context.insert("categories", &categories);

    match state.tera.render("index.html", &context) {
        Ok(html) => Ok(Html(html).into_response()),
        Err(e) => {
            error!("Template error: {}", e);
            Err(StatusCode::INTERNAL_SERVER_ERROR)
        }
    }
}

pub async fn feed_handler(
    State(state): State<Arc<AppState>>,
) -> Result<Json<Vec<Gallery>>, StatusCode> {
    info!("Generating Feed");

    let galleries = state.gallery_service.get_galleries().await;
    Ok(Json(galleries))
}

pub async fn page_handler(
    State(state): State<Arc<AppState>>,
    Path(stub): Path<String>,
) -> Result<Response, StatusCode> {
    let path = format!("/gallery/{}", stub);

    match state.gallery_service.get_gallery(&path).await {
        Ok(gallery) => {
            info!("Generating Gallery Page: {}", path);

            let mut context = Context::new();
            context.insert("name", &gallery.name);
            context.insert("category", &gallery.category);
            context.insert("videos", &gallery.videos);

            match state.tera.render("gallery.html", &context) {
                Ok(html) => Ok(Html(html).into_response()),
                Err(e) => {
                    error!("Template error: {}", e);
                    Err(StatusCode::INTERNAL_SERVER_ERROR)
                }
            }
        }
        Err(_) => {
            error!("Gallery not found: {}", path);
            Err(StatusCode::NOT_FOUND)
        }
    }
}
