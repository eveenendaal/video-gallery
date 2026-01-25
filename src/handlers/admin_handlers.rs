use crate::handlers::handlers::AppState;
use crate::models::{
    Admin, BulkClearRequest, BulkGenerateRequest, ClearThumbnailRequest,
    FetchMoviePosterRequest, GenerateThumbnailRequest, MoviePosterResult, ProgressUpdate,
};
use crate::services::{PosterService, ThumbnailService};
use axum::{
    extract::{Query, State},
    http::StatusCode,
    response::{Html, IntoResponse, Response, Sse},
    Json,
};
use axum::response::sse::Event;
use futures::stream::{self, Stream};
use serde::Deserialize;
use std::convert::Infallible;
use std::sync::Arc;
use std::time::Duration;
use tera::Context;
use tokio::sync::mpsc;
use tracing::{error, info};

pub async fn admin_handler(
    State(state): State<Arc<AppState>>,
) -> Result<Response, StatusCode> {
    info!("Generating Admin Page");

    let categories = state.gallery_service.get_categories().await;

    let mut context = Context::new();
    context.insert("categories", &categories);
    context.insert("secret_key", &state.config.secret_key);

    match state.tera.render("admin.html", &context) {
        Ok(html) => Ok(Html(html).into_response()),
        Err(e) => {
            error!("Template error: {}", e);
            Err(StatusCode::INTERNAL_SERVER_ERROR)
        }
    }
}

#[derive(Deserialize)]
pub struct GenerateThumbnailQuery {
    #[serde(rename = "videoPath")]
    video_path: String,
    #[serde(rename = "timeMs")]
    time_ms: Option<i32>,
}

pub async fn generate_thumbnail_handler(
    State(state): State<Arc<AppState>>,
    Query(params): Query<GenerateThumbnailQuery>,
) -> Sse<impl Stream<Item = Result<Event, Infallible>>> {
    let video_path = params.video_path;
    let time_ms = params.time_ms.unwrap_or(1000);

    info!("Generating thumbnail for video: {} at time: {}ms", video_path, time_ms);

    let thumbnail_service = ThumbnailService::new(state.config.clone());
    let gallery_service = state.gallery_service.clone();

    let (tx, rx) = mpsc::unbounded_channel();

    tokio::spawn(async move {
        let tx_clone = tx.clone();
        let progress_cb = Arc::new(move |step: String, progress: i32| {
            let update = ProgressUpdate {
                step,
                progress,
                error: None,
            };
            let _ = tx_clone.send(update);
        });

        let result = thumbnail_service
            .generate_thumbnail_with_progress(&video_path, time_ms, Some(progress_cb.clone()))
            .await;

        if let Err(e) = result {
            let update = ProgressUpdate {
                step: "Error".to_string(),
                progress: 0,
                error: Some(e.to_string()),
            };
            let _ = tx.send(update);
        } else {
            gallery_service.invalidate_cache().await;
        }
    });

    let stream = stream::unfold(rx, |mut rx| async move {
        rx.recv().await.map(|update| {
            let json = serde_json::to_string(&update).unwrap_or_default();
            let event = Event::default().data(json);
            (Ok(event), rx)
        })
    });

    Sse::new(stream).keep_alive(
        axum::response::sse::KeepAlive::new()
            .interval(Duration::from_secs(1))
            .text("keep-alive"),
    )
}

pub async fn clear_thumbnail_handler(
    State(state): State<Arc<AppState>>,
    Json(req): Json<ClearThumbnailRequest>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    info!("Clearing thumbnail: {}", req.thumbnail_path);

    let thumbnail_service = ThumbnailService::new(state.config.clone());

    match thumbnail_service.clear_thumbnail(&req.thumbnail_path).await {
        Ok(_) => {
            state.gallery_service.invalidate_cache().await;
            Ok(Json(serde_json::json!({ "success": true })))
        }
        Err(e) => {
            error!("Failed to clear thumbnail: {}", e);
            Err(StatusCode::INTERNAL_SERVER_ERROR)
        }
    }
}

pub async fn bulk_generate_thumbnails_handler(
    State(state): State<Arc<AppState>>,
    Json(req): Json<BulkGenerateRequest>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    info!("Bulk generating {} thumbnails", req.video_paths.len());

    let thumbnail_service = ThumbnailService::new(state.config.clone());
    let max_parallel = req.max_parallel.unwrap_or(3).min(10);

    let (success, failed) = thumbnail_service
        .bulk_generate_thumbnails(req.video_paths, req.time_ms, max_parallel)
        .await;

    state.gallery_service.invalidate_cache().await;

    Ok(Json(serde_json::json!({
        "success": success,
        "failed": failed
    })))
}

pub async fn bulk_clear_thumbnails_handler(
    State(state): State<Arc<AppState>>,
    Json(req): Json<BulkClearRequest>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    info!("Bulk clearing {} thumbnails", req.thumbnail_paths.len());

    let thumbnail_service = ThumbnailService::new(state.config.clone());

    let cleared = thumbnail_service
        .bulk_clear_thumbnails(req.thumbnail_paths)
        .await;

    state.gallery_service.invalidate_cache().await;

    Ok(Json(serde_json::json!({ "cleared": cleared })))
}

pub async fn fetch_movie_poster_handler(
    State(state): State<Arc<AppState>>,
    Query(params): Query<GenerateThumbnailQuery>,
) -> Sse<impl Stream<Item = Result<Event, Infallible>>> {
    let video_path = params.video_path;

    info!("Fetching movie poster for: {}", video_path);

    let poster_service = PosterService::new(state.config.clone());
    let gallery_service = state.gallery_service.clone();

    let (tx, rx) = mpsc::unbounded_channel();

    tokio::spawn(async move {
        let tx_clone = tx.clone();
        let progress_cb = Arc::new(move |step: String, progress: i32| {
            let update = ProgressUpdate {
                step,
                progress,
                error: None,
            };
            let _ = tx_clone.send(update);
        });

        let result = poster_service
            .fetch_movie_poster(&video_path, "", Some(progress_cb.clone()))
            .await;

        if let Err(e) = result {
            let update = ProgressUpdate {
                step: "Error".to_string(),
                progress: 0,
                error: Some(e.to_string()),
            };
            let _ = tx.send(update);
        } else {
            gallery_service.invalidate_cache().await;
        }
    });

    let stream = stream::unfold(rx, |mut rx| async move {
        rx.recv().await.map(|update| {
            let json = serde_json::to_string(&update).unwrap_or_default();
            let event = Event::default().data(json);
            (Ok(event), rx)
        })
    });

    Sse::new(stream).keep_alive(
        axum::response::sse::KeepAlive::new()
            .interval(Duration::from_secs(1))
            .text("keep-alive"),
    )
}

#[derive(Deserialize)]
pub struct SearchMovieQuery {
    #[serde(rename = "movieTitle")]
    movie_title: String,
}

pub async fn search_movie_poster_handler(
    State(state): State<Arc<AppState>>,
    Query(params): Query<SearchMovieQuery>,
) -> Result<Json<Vec<MoviePosterResult>>, StatusCode> {
    info!("Searching for movie: {}", params.movie_title);

    let poster_service = PosterService::new(state.config.clone());

    match poster_service.search_movie_poster(&params.movie_title).await {
        Ok(results) => Ok(Json(results)),
        Err(e) => {
            error!("Failed to search movie: {}", e);
            Err(StatusCode::INTERNAL_SERVER_ERROR)
        }
    }
}
